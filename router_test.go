package httprouter

import (
	"net/http"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	tree := newTreeNode()

	assertRoutes(t, tree,
		addRoute(tree, "GET", "/"),
		// addRoute(tree, "PUT", "/"), // TODO
	)
}
func addRoute(tree *treeNode, method, path string) fakeHandler {
	parsedMethod := ParseMethod(method)
	handler := newSampleHandler(parsedMethod, path)
	tree.Add(handler.Route())
	return handler
}
func assertRoutes(t *testing.T, tree *treeNode, handlers ...fakeHandler) {
	for _, handler := range handlers {
		route := handler.Route()
		resolved, _ := tree.Resolve(route.AllowedMethods, route.Path)
		Assert(t).That(resolved).Equals(handler)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type fakeHandler string

func newSampleHandler(method Method, path string) fakeHandler {
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
