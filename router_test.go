package httprouter

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouting(t *testing.T) {
	router := RequireNew(
		Options.Routes(
			ParseRoute("GET|HEAD ", "/test1/path/to/document", simpleHandler("1")),
			ParseRoute("POST     ", "/test1/path/to/document", simpleHandler("2")),
			ParseRoute("DELETE   ", "/test1/path/to/document", simpleHandler("3")),
			ParseRoute("PATCH    ", "/test1/path/to/document", simpleHandler("18")),

			ParseRoute("GET      ", "/test2/*               ", simpleHandler("4")),
			ParseRoute("PUT      ", "/test2/path/to/document", simpleHandler("5")),
			ParseRoute("DELETE   ", "/test2/:id/to/document ", simpleHandler("6")),

			ParseRoute("GET      ", "/:var1/:var2/test3/path/to/document", simpleHandler("7")),
			ParseRoute("PUT      ", "/:var1/:var2/test3/path/to/document", simpleHandler("8")),
			ParseRoute("GET      ", "/:var1/another/path/to/document    ", simpleHandler("9")),

			ParseRoute("CONNECT  ", "/test4  ", simpleHandler("10")),
			ParseRoute("CONNECT  ", "/test4/ ", simpleHandler("11")),
			ParseRoute("CONNECT  ", "/test4/*", simpleHandler("12")),

			ParseRoute("TRACE    ", "/test5/static/child/:variable/grandchild", simpleHandler("13")),
			ParseRoute("TRACE    ", "/test5/:variable/child/static/grandchild", simpleHandler("14")),
			ParseRoute("TRACE    ", "/test5/:variable/child/*                ", simpleHandler("15")),

			ParseRoute("GET      ", "/test5/:variable-1/:variable-2/:variable-3/static", simpleHandler("16")),
			ParseRoute("GET      ", "/test5/:variable-1/:variable-2/static/child      ", simpleHandler("17")),
		),
	)

	assertRoute(t, router, "GET    ", "/", 404, "Not Found\n", "")

	assertRoute(t, router, "GET    ", "/test1/path/to/document ", 200, "1", "")
	assertRoute(t, router, "GET    ", "/test1/path/to/document/", 404, "Not Found\n", "")
	assertRoute(t, router, "GET    ", "/test1/path/to/doc      ", 404, "Not Found\n", "")
	assertRoute(t, router, "GET    ", "/test1/path/to/         ", 404, "Not Found\n", "")
	assertRoute(t, router, "PUT    ", "/test1/path/to/document ", 405, "Method Not Allowed\n", "GET, HEAD, POST, DELETE, PATCH")
	assertRoute(t, router, "POST   ", "/test1/path/to/document ", 200, "2", "")
	assertRoute(t, router, "OPTIONS", "/test1/path/to/document ", 405, "Method Not Allowed\n", "GET, HEAD, POST, DELETE, PATCH")
	assertRoute(t, router, "DELETE ", "/test1/path/to/document ", 200, "3", "")
	assertRoute(t, router, "PATCH  ", "/test1/path/to/document ", 200, "18", "")
	assertRoute(t, router, "BOGUS  ", "/test1/path/to/document ", 405, "Method Not Allowed\n", "GET, HEAD, POST, DELETE, PATCH")

	assertRoute(t, router, "GET    ", "/test2/path/to/document               ", 200, "4", "")
	assertRoute(t, router, "PUT    ", "/test2/path/to/document               ", 200, "5", "")
	assertRoute(t, router, "DELETE ", "/test2/path/to/document               ", 200, "6", "")
	assertRoute(t, router, "PATCH  ", "/test2/path/to/document               ", 405, "Method Not Allowed\n", "GET, PUT, DELETE")
	assertRoute(t, router, "DELETE ", "/test2/path/to/document/does-not-exist", 405, "Method Not Allowed\n", "GET") // greedy GET /test2/*

	assertRoute(t, router, "GET    ", "/variable1/variable1/test3/path/to/document", 200, "7", "")

	assertRoute(t, router, "CONNECT", "/test4         ", 200, "10", "")
	assertRoute(t, router, "HEAD   ", "/test4         ", 405, "Method Not Allowed\n", "CONNECT")
	assertRoute(t, router, "CONNECT", "/test4/        ", 200, "11", "")
	assertRoute(t, router, "CONNECT", "/test4/wildcard", 200, "12", "")
	assertRoute(t, router, "DELETE ", "/test4/wildcard", 405, "Method Not Allowed\n", "CONNECT")

	assertRoute(t, router, "TRACE  ", "/test5/static/child/variable-name-here/grandchild               ", 200, "13", "")
	assertRoute(t, router, "TRACE  ", "/test5/static/child/variable-name-here/grandchild/does-not-exist", 200, "15", "") // greedy wildcard
	assertRoute(t, router, "TRACE  ", "/test5/variable-name-here/child/static/grandchild               ", 200, "14", "")
	assertRoute(t, router, "TRACE  ", "/test5/variable-name-here/child/wildcard                        ", 200, "15", "")

	assertRoute(t, router, "GET    ", "/test5/variable-1-here/variable-2-here/variable-3-here/static", 200, "16", "")
	assertRoute(t, router, "DELETE ", "/test5/variable-1-here/variable-2-here/variable-3-here/static", 405, "Method Not Allowed\n", "GET")
	assertRoute(t, router, "GET    ", "/test5/variable-1-here/variable-2-here/static/child          ", 200, "17", "")
}
func assertRoute(t *testing.T, router http.Handler, method, path string, expectedStatus int, expectedBody, expectedAllow string) {
	t.Helper()
	t.Run(fmt.Sprintf("%s:%s:%d", method, path, expectedStatus), func(t *testing.T) {
		t.Helper()

		request := httptest.NewRequest(strings.TrimSpace(method), strings.TrimSpace(path)+"?query=value#hash", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != expectedStatus {
			t.Errorf("expected status [%d], actual status: [%d] for test [%s %s]", expectedStatus, recorder.Code, method, path)
		} else {
			actualBody := recorder.Body.String()
			if actualBody != expectedBody {
				t.Errorf("expected body [%s], actual body: [%s] for test [%s %s]", expectedBody, actualBody, method, path)
			}
		}

		actualAllow := recorder.Header().Get("Allow")
		if actualAllow != expectedAllow {
			t.Errorf("expected Allow [%s], actual Allow: [%s] for test [%s %s]", expectedAllow, actualAllow, method, path)
		}
	})
}

// TestWideStaticNode exercises nodes with more than a handful of static children, which cross the threshold
// where the router switches from a linear scan to the map-backed index. Registers 12 siblings at the root and
// 12 more under a nested node, then verifies every route resolves to its own handler and that near-misses
// (unregistered segment, prefix of a segment, wrong method) behave correctly.
func TestWideStaticNode(t *testing.T) {
	segments := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot",
		"golf", "hotel", "india", "juliet", "kilo", "lima"} // 12 > threshold, forces the map branch

	options := make([]Route, 0, len(segments)*2)
	for _, segment := range segments {
		options = append(options, ParseRoute("GET", "/"+segment, simpleHandler(segment)))
		options = append(options, ParseRoute("GET", "/nested/"+segment, simpleHandler("nested-"+segment)))
	}
	router := RequireNew(Options.Routes(options...))

	for _, segment := range segments {
		assertRoute(t, router, "GET", "/"+segment, 200, segment, "")
		assertRoute(t, router, "GET", "/nested/"+segment, 200, "nested-"+segment, "")
	}

	assertRoute(t, router, "GET", "/mike", 404, "Not Found\n", "")     // unregistered wide sibling
	assertRoute(t, router, "GET", "/alph", 404, "Not Found\n", "")     // prefix of a registered segment
	assertRoute(t, router, "GET", "/alphabet", 404, "Not Found\n", "") // registered segment is a prefix
	assertRoute(t, router, "POST", "/alpha", 405, "Method Not Allowed\n", "GET")
}

// TestCompaction verifies that the post-registration compaction pass, which merges single-child static chains
// into multi-segment nodes, preserves routing semantics across every shape it can encounter: a long
// non-branching path, a shared prefix that branches, a static prefix ending in a variable or wildcard, an
// intermediate handler that must halt the merge, a trailing-slash leaf, and a wide node whose children are
// themselves compacted chains.
func TestCompaction(t *testing.T) {
	router := RequireNew(Options.Routes(
		ParseRoute("GET", "/api/v1/services/auth/tokens", simpleHandler("tokens")), // long non-branching
		ParseRoute("GET", "/shop/catalog/items/list", simpleHandler("list")),       // shared prefix, branches
		ParseRoute("GET", "/shop/catalog/items/detail", simpleHandler("detail")),   // ...at "items"
		ParseRoute("GET", "/users/profile/:id", simpleHandler("profile")),          // static prefix, variable leaf
		ParseRoute("GET", "/static/assets/*", simpleHandler("assets")),             // static prefix, wildcard leaf
		ParseRoute("GET", "/x/y", simpleHandler("xy")),                             // intermediate handler...
		ParseRoute("GET", "/x/y/z", simpleHandler("xyz")),                          // ...must not be swallowed
		ParseRoute("GET", "/trail/", simpleHandler("trail")),                       // trailing-slash leaf
	))

	assertRoute(t, router, "GET", "/api/v1/services/auth/tokens", 200, "tokens", "")
	assertRoute(t, router, "GET", "/api/v1/services/auth", 404, "Not Found\n", "") // intermediate, no handler
	assertRoute(t, router, "GET", "/api/v1/services/auth/tokensX", 404, "Not Found\n", "")
	assertRoute(t, router, "GET", "/shop/catalog/items/list", 200, "list", "")
	assertRoute(t, router, "GET", "/shop/catalog/items/detail", 200, "detail", "")
	assertRoute(t, router, "GET", "/shop/catalog/items", 404, "Not Found\n", "")
	assertRoute(t, router, "GET", "/users/profile/42", 200, "profile", "")
	assertRoute(t, router, "GET", "/users/profile", 404, "Not Found\n", "") // variable segment is required
	assertRoute(t, router, "GET", "/static/assets/css/app.min.css", 200, "assets", "")
	assertRoute(t, router, "GET", "/x/y", 200, "xy", "")    // handler on the intermediate node
	assertRoute(t, router, "GET", "/x/y/z", 200, "xyz", "") // deeper handler still reachable
	assertRoute(t, router, "GET", "/trail/", 200, "trail", "")
	assertRoute(t, router, "GET", "/trail", 404, "Not Found\n", "")
	assertRoute(t, router, "POST", "/x/y", 405, "Method Not Allowed\n", "GET")
}

// TestCompactionWideMultiSegment exercises a wide node (map-backed index) whose children are themselves
// compacted multi-segment chains — the branch where the first-segment map probe must still confirm the rest of
// the fused fragment. Registers enough siblings to cross staticIndexThreshold, each a two-segment chain.
func TestCompactionWideMultiSegment(t *testing.T) {
	segments := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot",
		"golf", "hotel", "india", "juliet"} // 10 > threshold, forces the map branch

	options := make([]Route, 0, len(segments))
	for _, segment := range segments {
		options = append(options, ParseRoute("GET", "/"+segment+"/detail", simpleHandler(segment)))
	}
	router := RequireNew(Options.Routes(options...))

	for _, segment := range segments {
		assertRoute(t, router, "GET", "/"+segment+"/detail", 200, segment, "")
	}
	assertRoute(t, router, "GET", "/alpha", 404, "Not Found\n", "")         // first segment only: fused tail unmatched
	assertRoute(t, router, "GET", "/alpha/summary", 404, "Not Found\n", "") // right first segment, wrong tail
	assertRoute(t, router, "GET", "/alpha/detailX", 404, "Not Found\n", "") // tail is a prefix, boundary rejects
	assertRoute(t, router, "GET", "/mike/detail", 404, "Not Found\n", "")   // unregistered first segment
}

func TestVariableRequiresNonEmptySegment(t *testing.T) {
	router := RequireNew(
		Options.Routes(
			ParseRoute("GET", "/users/:id", simpleHandler("user")),
			ParseRoute("GET", "/users/:id/profile", simpleHandler("profile")),
		),
	)

	assertRoute(t, router, "GET", "/users/42", 200, "user", "")
	assertRoute(t, router, "GET", "/users/42/profile", 200, "profile", "")
	assertRoute(t, router, "GET", "/users/", 404, "Not Found\n", "")
	assertRoute(t, router, "GET", "/users//", 404, "Not Found\n", "")
	assertRoute(t, router, "GET", "/users//profile", 404, "Not Found\n", "")
	assertRoute(t, router, "POST", "/users/", 404, "Not Found\n", "")
}

// TestCompactionCollapsesNodes asserts the pass actually merges the chain (not merely that routing still works),
// and that an intermediate handler halts the merge so the node retains both its handler and its deeper child.
func TestCompactionCollapsesNodes(t *testing.T) {
	tree := &treeNode{}
	addRoute(tree, "GET", "/alpha/beta/gamma")
	tree.compact()

	Assert(t).That(len(tree.static)).Equals(1)
	Assert(t).That(tree.static[0].pathFragment).Equals("alpha/beta/gamma") // three segments fused into one node

	branching := &treeNode{}
	addRoute(branching, "GET", "/x/y")
	addRoute(branching, "GET", "/x/y/z")
	branching.compact()

	merged := branching.static[0]
	Assert(t).That(merged.pathFragment).Equals("x/y") // prefix fused, but the merge stops at the /x/y handler
	if merged.handlers == nil {
		t.Error("expected the /x/y handler to survive on the merged node")
	}
	Assert(t).That(len(merged.static)).Equals(1)
	Assert(t).That(merged.static[0].pathFragment).Equals("z")
}

func TestFallbackToURL(t *testing.T) {
	router := RequireNew(Options.AddRoute("GET", "/", simpleHandler(t.Name())))
	request := httptest.NewRequest("GET", "/", nil)
	request.RequestURI = "" // causes fallback to request.URL
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("expected status [%d], actual status: [%d]", http.StatusOK, recorder.Code)
	}
}

func TestRecovery(t *testing.T) {
	handler := RequireNew(
		Options.AddRoute("GET", "/*", simpleHandler("500")),
		Options.Recovery(RecoveryHandler))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, nil)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("expected status [%d], actual status: [%d]", http.StatusInternalServerError, recorder.Code)
	}
}
func TestRequireNew_WillPanic(t *testing.T) {
	var fatal bool

	func() {
		defer func() { fatal = recover() != nil }()
		_ = RequireNew(Options.AddRoute("BAD-METHOD", "/*wildcard", simpleHandler("")))
	}()

	Assert(t).That(fatal).Equals(true)
}

func TestRouteAlreadyExists(t *testing.T) {
	assertRouteAlreadyExists(t, "GET")
	assertRouteAlreadyExists(t, "HEAD")
	assertRouteAlreadyExists(t, "POST")
	assertRouteAlreadyExists(t, "PUT")
	assertRouteAlreadyExists(t, "DELETE")
	assertRouteAlreadyExists(t, "CONNECT")
	assertRouteAlreadyExists(t, "OPTIONS")
	assertRouteAlreadyExists(t, "TRACE")
	assertRouteAlreadyExists(t, "PATCH")
}
func assertRouteAlreadyExists(t *testing.T, method string) {
	t.Run(method, func(t *testing.T) {
		tree := &treeNode{}
		_, err1 := addRouteWithError(tree, method, "/stuff")
		_, err2 := addRouteWithError(tree, method, "/stuff")
		Assert(t).That(err1).IsNil()
		Assert(t).That(err2).Equals(ErrRouteExists)
	})
}
func TestRouteAlreadyExists_NoPartialRegistration(t *testing.T) {
	tree := &treeNode{}
	handler := simpleHandler("original")

	_ = tree.Add(Route{AllowedMethods: MethodPost, Path: "/stuff", Handler: handler})
	err := tree.Add(Route{AllowedMethods: MethodGet | MethodPost, Path: "/stuff", Handler: handler})

	Assert(t).That(err).Equals(ErrRouteExists)

	resolved, _ := tree.Resolve("GET", "/stuff")
	if resolved != nil {
		t.Error("expected GET handler to be nil, but it was registered")
	}
}
func TestNilRouteHandler(t *testing.T) {
	tree := &treeNode{}
	route := Route{AllowedMethods: MethodGet, Path: "/stuff"}

	Assert(t).That(tree.Add(route)).Equals(ErrNilHandler)

	route.Handler = simpleHandler(t.Name())
	Assert(t).That(tree.Add(route)).IsNil()

	handler, _ := tree.Resolve("GET", "/stuff")
	if handler == nil {
		t.Error("expected valid registration after rejected nil handler")
	}

	_, err := New(Options.AddRoute("GET", "/stuff", nil))
	Assert(t).That(err).Equals(ErrNilHandler)
}
func TestNonASCIICharactersRejected(t *testing.T) {
	tree := &treeNode{}
	_, err1 := addRouteWithError(tree, "GET", "/café")
	_, err2 := addRouteWithError(tree, "GET", "/日本語")
	_, err3 := addRouteWithError(tree, "GET", "/path/über")
	Assert(t).That(err1).Equals(ErrInvalidCharacters)
	Assert(t).That(err2).Equals(ErrInvalidCharacters)
	Assert(t).That(err3).Equals(ErrInvalidCharacters)
}
func TestMalformedRouteRegistration(t *testing.T) {
	tree := &treeNode{}
	_, err1 := addRouteWithError(tree, "GET", "//stuff")
	_, err2 := addRouteWithError(tree, "GET", "/stu*ff")
	_, err3 := addRouteWithError(tree, "GET", "/stu:ff")
	_, err4 := addRouteWithError(tree, "GET", "/stuff//identities")
	_, err5 := addRouteWithError(tree, "GET", "/stuff/*more_stuff")
	_, err6 := addRouteWithError(tree, "GET", "stuff")
	_, err7 := addRouteWithError(tree, "BAD-METHOD", "/")
	Assert(t).That(err1).Equals(ErrMalformedPath)
	Assert(t).That(err2).Equals(ErrInvalidCharacters)
	Assert(t).That(err3).Equals(ErrInvalidCharacters)
	Assert(t).That(err4).Equals(ErrMalformedPath)
	Assert(t).That(err5).Equals(ErrInvalidWildcard)
	Assert(t).That(err6).Equals(ErrMalformedPath)
	Assert(t).That(err7).Equals(ErrUnknownMethod)
}

func addRoute(tree *treeNode, method, path string) fakeHandler {
	parsedMethod := ParseMethod(method)
	handler := newFakeHandler(parsedMethod, path)

	_ = tree.Add(handler.Route())
	return handler
}
func addRouteWithError(tree *treeNode, method, path string) (fakeHandler, error) {
	parsedMethod := ParseMethod(method)
	handler := newFakeHandler(parsedMethod, path)

	err := tree.Add(handler.Route())
	return handler, err
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type fakeHandler string

func newFakeHandler(method Method, path string) fakeHandler {
	return fakeHandler(method.String() + " " + path)
}
func (this fakeHandler) Route() Route {
	return Route{
		AllowedMethods: ParseMethod(strings.Split(string(this), " ")[0]),
		Path:           strings.Split(string(this), " ")[1],
		Handler:        this,
	}
}
func (this fakeHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

type simpleHandler string

func (this simpleHandler) ServeHTTP(response http.ResponseWriter, _ *http.Request) {
	if this == "500" {
		panic("500")
	} else {
		response.WriteHeader(200)
		_, _ = io.WriteString(response, string(this))
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func BenchmarkTreeStatic(b *testing.B) {
	tree := &treeNode{}
	addRoute(tree, "GET", "/")
	addRoute(tree, "GET", "/stuff")
	addRoute(tree, "GET", "/stuff/identities")
	addRoute(tree, "GET", "/stuff/identities/long/path")
	addRoute(tree, "GET", "/stuff1")
	tree.compact()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("GET", "/") // slows down as the node traversal gets longer
	}
}
func BenchmarkRouter(b *testing.B) {
	router := RequireNew(
		Options.Routes(
			ParseRoute("GET", "/child1/node/", &nopHandler{}),
			ParseRoute("GET", "/child2/node", &nopHandler{}),
			ParseRoute("GET", "/child3/node", &nopHandler{}),
			ParseRoute("GET", "/path", &nopHandler{}),
		))

	request := httptest.NewRequest("GET", "/path", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(nil, request)
	}
}

func BenchmarkTreeWide(b *testing.B) {
	const childCount = 100
	tree := &treeNode{}
	for i := 0; i < childCount; i++ {
		addRoute(tree, "GET", fmt.Sprintf("/segment%d", i))
	}
	tree.compact()

	// Resolve the last-registered child: worst case for a linear scan of the static children.
	lastChild := fmt.Sprintf("/segment%d", childCount-1)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("GET", lastChild)
	}
}
func BenchmarkTreeVariable(b *testing.B) {
	tree := &treeNode{}
	addRoute(tree, "GET", "/stuff/:id/details")
	tree.compact()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = tree.Resolve("GET", "/stuff/42/details")
	}
}
func BenchmarkRouterQuery(b *testing.B) {
	router := RequireNew(
		Options.Routes(
			ParseRoute("GET", "/child1/node/", &nopHandler{}),
			ParseRoute("GET", "/child2/node", &nopHandler{}),
			ParseRoute("GET", "/child3/node", &nopHandler{}),
			ParseRoute("GET", "/path", &nopHandler{}),
		))

	request := httptest.NewRequest("GET", "/path?query=value&another=thing", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(nil, request)
	}
}

type nopHandler struct{}

func (this *nopHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
