package httprouter

import (
	"net/http"
	"strings"
	"testing"
)

func TestStaticRoutes(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.static)
	assertRoutes(t, tree,
		addRoute(tree, "GET", "/"),
		addRoute(tree, "GET", "/stuff"),
		addRoute(tree, "GET", "/stuff/identities"),
		addRoute(tree, "GET", "/stuff1"),
	)
	Assert(t).That(len(tree.static)).Equals(numOfStaticChildren + 3)
}
func TestVariableRoutes(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.static)
	assertRoutes(t, tree,
		addRoute(tree, "GET", "/stuff/:id"),
		addRoute(tree, "GET", "/stuff/identities/:id"),
	)
	Assert(t).That(len(tree.static)).Equals(numOfStaticChildren + 1)
}
func TestWildcardRoutes(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.static)
	assertRoutes(t, tree,
		addRoute(tree, "GET", "/stuff/identities/*"),
	)
	Assert(t).That(len(tree.static)).Equals(numOfStaticChildren + 1)
}

func TestMethods(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.static)
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
	Assert(t).That(len(tree.static)).Equals(numOfStaticChildren + 1)
}
func TestHandlers(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.static)
	assertRoutes(t, tree,
		addRoute(tree, "GET", "/stuff"),
		addRoute(tree, "POST", "/stuff"),
	)
	assertNonExistingRoute(t, tree,
		createNonExistingRoute("DELETE", "/stuff"))
	Assert(t).That(len(tree.static)).Equals(numOfStaticChildren + 1)
}
func TestHandlerAlreadyExistsErr(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.static)
	_, err1 := addRouteWithError(tree, "GET", "/stuff")
	_, err2 := addRouteWithError(tree, "GET", "/stuff")
	Assert(t).That(len(tree.static)).Equals(numOfStaticChildren + 1)
	Assert(t).That(err1).Equals(nil)
	Assert(t).That(err2).Equals(ErrRouteExists)
}
func TestMalformedRouteErr(t *testing.T) {
	tree := &treeNode{}
	numOfStaticChildren := len(tree.static)
	_, err1 := addRouteWithError(tree, "GET", "//stuff")
	_, err2 := addRouteWithError(tree, "GET", "/stu*ff")
	_, err3 := addRouteWithError(tree, "GET", "/stu:ff")
	_, err4 := addRouteWithError(tree, "GET", "/stuff//identities")
	_, err5 := addRouteWithError(tree, "GET", "/stuff/*more_stuff")
	_, err6 := addRouteWithError(tree, "GET", "stuff")
	Assert(t).That(len(tree.static)).Equals(numOfStaticChildren)
	Assert(t).That(err1).Equals(ErrMalformedPath)
	Assert(t).That(err2).Equals(ErrInvalidCharacters)
	Assert(t).That(err3).Equals(ErrInvalidCharacters)
	Assert(t).That(err4).Equals(ErrMalformedPath)
	Assert(t).That(err5).Equals(ErrInvalidWildcard)
	Assert(t).That(err6).Equals(ErrMalformedPath)
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
func addRouteWithError(tree *treeNode, method, path string) (fakeHandler, error) {
	parsedMethod := ParseMethod(method)
	handler := newSampleHandler(parsedMethod, path)

	err := tree.Add(handler.Route())
	return handler, err
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
