package httprouter

import (
	"net/http"
	"strings"
	"testing"
)

func TestStaticRoutes(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.staticChildren)
	assertRoutes(t, tree,
		addRoute(tree, "GET", "/"),
		addRoute(tree, "GET", "/stuff"),
		addRoute(tree, "GET", "/stuff/identities"),
	)
	Assert(t).That(len(tree.staticChildren)).Equals(numOfStaticChildren + 2)
}

func TestVariableRoutes(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.staticChildren)
	assertRoutes(t, tree,
		addRoute(tree, "GET", "/stuff/:id"),
		addRoute(tree, "GET", "/stuff/identities/:id"),
	)
	Assert(t).That(len(tree.staticChildren)).Equals(numOfStaticChildren + 1)
}

func TestWildcardRoutes(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.staticChildren)
	assertRoutes(t, tree,
		addRoute(tree, "GET", "/stuff/identities/*"),
	)
	Assert(t).That(len(tree.staticChildren)).Equals(numOfStaticChildren + 1)
}

func TestMethods(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.staticChildren)
	assertRoutes(t, tree,
		addRoute(tree, "GET", "/stuff"),
		addRoute(tree, "DELETE", "/stuff"),
		addRoute(tree, "PUT", "/stuff"),
		addRoute(tree, "POST", "/stuff"),

		addRoute(tree, "GET", "/stuff/:id"),
		addRoute(tree, "DELETE", "/stuff/:id"),
		addRoute(tree, "PUT", "/stuff/:id"),
		addRoute(tree, "POST", "/stuff/:id"),

		addRoute(tree, "GET", "/stuff/*"),
		addRoute(tree, "DELETE", "/stuff/*"),
		addRoute(tree, "PUT", "/stuff/*"),
		addRoute(tree, "POST", "/stuff/*"),
	)
	Assert(t).That(len(tree.staticChildren)).Equals(numOfStaticChildren + 1)
}

func TestHandlers(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.staticChildren)
	assertRoutes(t, tree,
		addRoute(tree, "GET", "/stuff"),
		addRoute(tree, "POST", "/stuff"),
	)
	assertNonExistingRoute(t, tree,
		createNonExistingRoute("DELETE", "/stuff"))
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
func createNonExistingRoute(method, path string) fakeHandler {
	parsedMethod := ParseMethod(method)
	handler := newSampleHandler(parsedMethod, path)

	return handler
}
func assertNonExistingRoute(t *testing.T, tree *treeNode, handlers ...fakeHandler) {
	for _, handler := range handlers {
		route := handler.Route()
		resolved, _ := tree.Resolve(route.AllowedMethod, route.Path)
		Assert(t).That(resolved).Equals(nil)
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
