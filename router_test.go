package httprouter

import (
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

			ParseRoute("GET      ", "/test2/*               ", simpleHandler("4")),
			ParseRoute("PUT      ", "/test2/path/to/document", simpleHandler("5")),
			ParseRoute("DELETE   ", "/test2/:id/to/document ", simpleHandler("6")),

			ParseRoute("GET      ", "/:var1/:var2/test3/path/to/document", simpleHandler("7")),
			ParseRoute("PUT      ", "/:var1/:var2/test3/path/to/document", simpleHandler("8")),
			ParseRoute("GET      ", "/:var1/another/path/to/document    ", simpleHandler("9")),

			ParseRoute("GET      ", "/test4  ", simpleHandler("10")),
			ParseRoute("GET      ", "/test4/ ", simpleHandler("11")),
			ParseRoute("GET      ", "/test4/*", simpleHandler("12")),

			ParseRoute("GET      ", "/test5/static/child/:variable/grandchild", simpleHandler("13")),
			ParseRoute("GET      ", "/test5/:variable/child/static/grandchild", simpleHandler("14")),
			ParseRoute("GET      ", "/test5/:variable/child/*                ", simpleHandler("15")),

			ParseRoute("GET      ", "/test5/:variable-1/:variable-2/:variable-3/static", simpleHandler("16")),
			ParseRoute("GET      ", "/test5/:variable-1/:variable-2/static/child      ", simpleHandler("17")),
		),
	)

	assertRoute(t, router, "GET    ", "/", 404, "Not Found\n")

	assertRoute(t, router, "GET    ", "/test1/path/to/document ", 200, "1")
	assertRoute(t, router, "GET    ", "/test1/path/to/document/", 404, "Not Found\n")
	assertRoute(t, router, "GET    ", "/test1/path/to/doc      ", 404, "Not Found\n")
	assertRoute(t, router, "GET    ", "/test1/path/to/         ", 404, "Not Found\n")
	assertRoute(t, router, "PUT    ", "/test1/path/to/document ", 405, "Method Not Allowed\n")
	assertRoute(t, router, "POST   ", "/test1/path/to/document ", 200, "2")
	assertRoute(t, router, "OPTIONS", "/test1/path/to/document ", 405, "Method Not Allowed\n")
	assertRoute(t, router, "DELETE ", "/test1/path/to/document ", 200, "3")

	assertRoute(t, router, "GET    ", "/test2/path/to/document               ", 200, "4")
	assertRoute(t, router, "PUT    ", "/test2/path/to/document               ", 200, "5")
	assertRoute(t, router, "DELETE ", "/test2/path/to/document               ", 200, "6")
	assertRoute(t, router, "PATCH  ", "/test2/path/to/document               ", 405, "Method Not Allowed\n")
	assertRoute(t, router, "DELETE ", "/test2/path/to/document/does-not-exist", 405, "Method Not Allowed\n") // greedy GET /test2/*

	assertRoute(t, router, "GET    ", "/variable1/variable1/test3/path/to/document", 200, "7")

	assertRoute(t, router, "GET    ", "/test4         ", 200, "10")
	assertRoute(t, router, "HEAD   ", "/test4         ", 405, "Method Not Allowed\n")
	assertRoute(t, router, "GET    ", "/test4/        ", 200, "11")
	assertRoute(t, router, "GET    ", "/test4/wildcard", 200, "12")
	assertRoute(t, router, "DELETE ", "/test4/wildcard", 405, "Method Not Allowed\n")

	assertRoute(t, router, "GET    ", "/test5/static/child/variable-name-here/grandchild               ", 200, "13")
	assertRoute(t, router, "GET    ", "/test5/static/child/variable-name-here/grandchild/does-not-exist", 200, "15") // greedy wildcard
	assertRoute(t, router, "GET    ", "/test5/variable-name-here/child/static/grandchild               ", 200, "14")
	assertRoute(t, router, "GET    ", "/test5/variable-name-here/child/wildcard                        ", 200, "15")

	assertRoute(t, router, "GET    ", "/test5/variable-1-here/variable-2-here/variable-3-here/static", 200, "16")
	assertRoute(t, router, "DELETE ", "/test5/variable-1-here/variable-2-here/variable-3-here/static", 405, "Method Not Allowed\n")
	assertRoute(t, router, "GET    ", "/test5/variable-1-here/variable-2-here/static/child          ", 200, "17")
}
func assertRoute(t *testing.T, router http.Handler, method, path string, expectedStatus int, expectedBody string) {
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
	tree := &treeNode{}
	_, err1 := addRouteWithError(tree, "GET", "/stuff")
	_, err2 := addRouteWithError(tree, "GET", "/stuff")
	Assert(t).That(err1).IsNil()
	Assert(t).That(err2).Equals(ErrRouteExists)
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

type nopHandler struct{}

func (this *nopHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
