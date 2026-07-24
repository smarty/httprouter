package httprouter

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Deep pure-static path: exercises recursion depth / per-level cost (segment match, slash strip, frames)
// before compaction fuses the chain, and the single fused-fragment comparison after.
func BenchmarkTreeDeepStatic(b *testing.B) {
	tree := &treeNode{}
	addRoute(tree, "GET", "/a/b/c/d/e/f/g/h/i/j")
	tree.compact()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("GET", "/a/b/c/d/e/f/g/h/i/j")
	}
}

// Narrow static node with same-length siblings BELOW staticIndexThreshold: the real linear-scan path
// (BenchmarkTreeWide's 100 children trip the map instead). Resolve the last sibling: worst case for the scan.
func BenchmarkTreeNarrowStatic(b *testing.B) {
	tree := &treeNode{}
	addRoute(tree, "GET", "/alpha")
	addRoute(tree, "GET", "/bravo")
	addRoute(tree, "GET", "/charl")
	addRoute(tree, "GET", "/delta")
	addRoute(tree, "GET", "/echoo")
	addRoute(tree, "GET", "/foxtr")
	tree.compact()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("GET", "/foxtr")
	}
}

// Wildcard resolution with a multi-segment tail.
func BenchmarkTreeWildcard(b *testing.B) {
	tree := &treeNode{}
	addRoute(tree, "GET", "/assets/*")
	tree.compact()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("GET", "/assets/css/vendor/app.min.css")
	}
}

// 405 at the tree level: exercises the static|variable allowed-bitmask OR-merge (no ResponseWriter noise).
func BenchmarkTreeMethodNotAllowed(b *testing.B) {
	tree := &treeNode{}
	addRoute(tree, "GET", "/users/:id")
	addRoute(tree, "POST", "/users/:id")
	tree.compact()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("DELETE", "/users/42")
	}
}

// Method.HeaderValue in isolation: the one per-request allocation on the 405 path (strings.Builder).
func BenchmarkHeaderValue(b *testing.B) {
	allowed := MethodGet | MethodHead | MethodPost | MethodPut

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = allowed.HeaderValue()
	}
}

// Full 405 through the router, real recorder (captures Header().Set("Allow", ...) + HeaderValue alloc).
func BenchmarkRouterMethodNotAllowed(b *testing.B) {
	router := RequireNew(
		Options.Routes(
			ParseRoute("GET|HEAD|POST", "/users/:id", &nopHandler{}),
		))
	request := httptest.NewRequest("DELETE", "/users/42", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
	}
}

// Realistic GitHub-like REST table with mixed static/variable/wildcard routes.
func benchRESTRouter() http.Handler {
	return RequireNew(
		Options.Routes(
			ParseRoute("GET", "/repos/:owner/:repo", &nopHandler{}),
			ParseRoute("GET", "/repos/:owner/:repo/issues", &nopHandler{}),
			ParseRoute("GET", "/repos/:owner/:repo/issues/:number", &nopHandler{}),
			ParseRoute("POST", "/repos/:owner/:repo/issues", &nopHandler{}),
			ParseRoute("GET", "/repos/:owner/:repo/pulls", &nopHandler{}),
			ParseRoute("GET", "/repos/:owner/:repo/pulls/:number", &nopHandler{}),
			ParseRoute("GET", "/repos/:owner/:repo/commits/:sha", &nopHandler{}),
			ParseRoute("GET", "/repos/:owner/:repo/contents/*", &nopHandler{}),
			ParseRoute("GET", "/users/:user", &nopHandler{}),
			ParseRoute("GET", "/users/:user/repos", &nopHandler{}),
			ParseRoute("GET", "/orgs/:org", &nopHandler{}),
			ParseRoute("GET", "/orgs/:org/members", &nopHandler{}),
			ParseRoute("GET", "/search/repositories", &nopHandler{}),
			ParseRoute("GET", "/search/issues", &nopHandler{}),
		))
}
func BenchmarkRESTDeepVariable(b *testing.B) {
	router := benchRESTRouter()
	request := httptest.NewRequest("GET", "/repos/smarty/httprouter/issues/42", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(nil, request)
	}
}
func BenchmarkRESTStatic(b *testing.B) {
	router := benchRESTRouter()
	request := httptest.NewRequest("GET", "/search/repositories", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(nil, request)
	}
}
func BenchmarkRESTWildcard(b *testing.B) {
	router := benchRESTRouter()
	request := httptest.NewRequest("GET", "/repos/smarty/httprouter/contents/src/main/go/router.go", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(nil, request)
	}
}

// Cycles request methods to expose the branchy tail of the method switch (array-index candidates target this).
func BenchmarkRouterMethodMix(b *testing.B) {
	router := RequireNew(
		Options.Routes(
			ParseRoute("GET|POST|PUT|DELETE|PATCH", "/path", &nopHandler{}),
		))
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	requests := make([]*http.Request, len(methods))
	for i, method := range methods {
		requests[i] = httptest.NewRequest(method, "/path", nil)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(nil, requests[i%len(requests)])
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Long-path benchmarks target radix path compression. Today every '/' becomes its own treeNode, so a deep
// non-branching path pays one recursive Resolve frame per segment. A compaction pass that merges single-child
// static chains (no handler, no variable, no wildcard) into one multi-segment node would settle each of these
// in a single comparison rather than a per-segment walk.

// buildLongStaticPath returns "/segment00/segment01/.../segmentNN" with segmentCount segments.
func buildLongStaticPath(segmentCount int) string {
	var builder strings.Builder
	for i := 0; i < segmentCount; i++ {
		_, _ = fmt.Fprintf(&builder, "/segment%02d", i)
	}
	return builder.String()
}

// Deep pure-static path with no competing pathways: the canonical case for path compression. 20 realistic
// multi-character segments, so the merged-node comparison cost (memequal over the whole fragment) is exercised,
// not merely the reduction in recursion frames.
func BenchmarkTreeLongStatic(b *testing.B) {
	const segmentCount = 20
	path := buildLongStaticPath(segmentCount)
	tree := &treeNode{}
	addRoute(tree, "GET", path)
	tree.compact()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("GET", path)
	}
}

// Long static prefix terminating in a variable segment: compression should collapse the static prefix even
// though the leaf is parametric, so the per-segment walk cost of the prefix disappears.
func BenchmarkTreeLongPrefixVariable(b *testing.B) {
	tree := &treeNode{}
	addRoute(tree, "GET", "/api/v1/services/authentication/providers/oauth2/tokens/:token")
	tree.compact()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("GET", "/api/v1/services/authentication/providers/oauth2/tokens/abc123")
	}
}

// Long shared static prefix, then a branch into several siblings — the realistic "every request pays the
// common-prefix walk" shape. All traffic under the prefix benefits when the prefix collapses to one node.
func BenchmarkTreeLongSharedPrefix(b *testing.B) {
	const prefix = "/api/v1/internal/platform/services/catalog"
	tree := &treeNode{}
	for _, leaf := range []string{"users", "orders", "invoices", "payments", "shipments"} {
		addRoute(tree, "GET", prefix+"/"+leaf)
	}
	tree.compact()
	request := prefix + "/shipments" // last-registered leaf: worst case for the branch scan

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("GET", request)
	}
}

// Wide node (map-backed) whose children are compacted two-segment chains: exercises the first-segment map
// probe followed by the fused-fragment confirmation. Resolve the last-registered child.
func BenchmarkTreeWideCompacted(b *testing.B) {
	const childCount = 100
	tree := &treeNode{}
	for i := 0; i < childCount; i++ {
		addRoute(tree, "GET", fmt.Sprintf("/segment%d/detail", i))
	}
	tree.compact()

	lastChild := fmt.Sprintf("/segment%d/detail", childCount-1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("GET", lastChild)
	}
}

// End-to-end ServeHTTP over a long static path: captures the router-level cost (path extraction + method
// dispatch) layered on top of the tree walk.
func BenchmarkRouterLongStatic(b *testing.B) {
	const segmentCount = 20
	path := buildLongStaticPath(segmentCount)
	router := RequireNew(Options.AddRoute("GET", path, &nopHandler{}))
	request := httptest.NewRequest("GET", path, nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(nil, request)
	}
}
