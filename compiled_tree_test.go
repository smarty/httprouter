package httprouter

import (
	"net/http"
	"reflect"
	"testing"
)

func TestCompiledTreeMatchesTree(t *testing.T) {
	tree := &treeNode{}
	routes := []Route{
		ParseRoute("GET|HEAD", "/static/path", simpleHandler("static")),
		ParseRoute("POST", "/static/:id", simpleHandler("variable")),
		ParseRoute("PUT", "/static/*", simpleHandler("wildcard")),
		ParseRoute("DELETE", "/:first/:second/end", simpleHandler("variables")),
		ParseRoute("PATCH", "/branch/fixed", simpleHandler("fixed")),
		ParseRoute("GET", "/branch/:value", simpleHandler("fallback-variable")),
		ParseRoute("POST", "/branch/*", simpleHandler("fallback-wildcard")),
		ParseRoute("CONNECT", "/slash/", simpleHandler("trailing-slash")),
	}
	for _, route := range routes {
		if err := tree.Add(route); err != nil {
			t.Fatal(err)
		}
	}
	tree.compact()
	compiled := compileTree(tree)

	testCases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/static/path"},
		{http.MethodPost, "/static/123"},
		{http.MethodPut, "/static/any/remaining/path"},
		{http.MethodDelete, "/one/two/end"},
		{http.MethodGet, "/branch/fixed"},    // static method misses, variable method matches
		{http.MethodPost, "/branch/fixed"},   // static and variable methods miss, wildcard matches
		{http.MethodDelete, "/branch/fixed"}, // allowed methods are combined across all matching shapes
		{http.MethodConnect, "/slash/"},
		{http.MethodConnect, "/slash"},
		{http.MethodGet, "/not-found"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			expectedHandler, expectedAllowed := tree.Resolve(testCase.method, testCase.path)
			actualHandler, actualAllowed := compiled.Resolve(testCase.method, testCase.path)

			if !reflect.DeepEqual(actualHandler, expectedHandler) {
				t.Fatalf("handler mismatch: expected %#v, actual %#v", expectedHandler, actualHandler)
			}
			if actualAllowed != expectedAllowed {
				t.Fatalf("allowed mismatch: expected %s, actual %s", expectedAllowed, actualAllowed)
			}
		})
	}
}

func BenchmarkResolverRepresentation(b *testing.B) {
	routes := []Route{
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
	}

	tree := &treeNode{}
	for _, route := range routes {
		if err := tree.Add(route); err != nil {
			b.Fatal(err)
		}
	}
	tree.compact()
	compiled := compileTree(tree)

	benchmarks := []struct {
		name string
		path string
	}{
		{name: "static", path: "/search/repositories"},
		{name: "deep-variable", path: "/repos/smarty/httprouter/issues/42"},
		{name: "wildcard", path: "/repos/smarty/httprouter/contents/src/main/go/router.go"},
	}
	resolvers := []struct {
		name     string
		resolver routeResolver
	}{
		{name: "pointer", resolver: tree},
		{name: "compiled", resolver: compiled},
	}

	for _, benchmark := range benchmarks {
		for _, resolver := range resolvers {
			b.Run(benchmark.name+"/"+resolver.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, _ = resolver.resolver.Resolve(http.MethodGet, benchmark.path)
				}
			})
		}
	}

	for _, benchmark := range benchmarks {
		for _, resolver := range resolvers {
			b.Run("router/"+benchmark.name+"/"+resolver.name, func(b *testing.B) {
				router := newRouter(resolver.resolver, &nopHandler{}, &nopHandler{}, &nop{})
				request := &http.Request{Method: http.MethodGet, RequestURI: benchmark.path}

				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					router.ServeHTTP(nil, request)
				}
			})
		}
	}
}
