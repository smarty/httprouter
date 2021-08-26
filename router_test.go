package httprouter

import (
	"net/http"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.staticChildren)
	assertRoutes(t, tree,
		addRoute(tree, "GET", "/stuff/identities"), //FIXME: How do we fix it with the leading /
		// addRoute(tree, "PUT", "/"), // TODO
	)
	Assert(t).That(len(tree.staticChildren)).Equals(numOfStaticChildren + 1)
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
		resolved, _ := tree.Resolve(route.AllowedMethod, route.Path)
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
		AllowedMethod: ParseMethod(strings.Split(string(this), " ")[0]),
		Path:          strings.Split(string(this), " ")[1],
		Handler:       this,
	}
}
func (this fakeHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
