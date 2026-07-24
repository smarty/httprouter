package httprouter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Deep pure-static path: exercises recursion depth / per-level cost (parsePathFragment, slash strip, frames).
func BenchmarkTreeDeepStatic(b *testing.B) {
	tree := &treeNode{}
	addRoute(tree, "GET", "/a/b/c/d/e/f/g/h/i/j")

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
