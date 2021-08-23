package httprouter

import (
	"net/http"
	"strings"
)

type routeResolver interface {
	// Resolve returns an instance of http.Handler and with a flag indicating if the route was understood.
	// If the http.Handler instance is not nil, the route was fully resolved and can be invoked.
	// If the http.Handler instance is nil AND the flag is true, the route was found, but the method isn't compatible (e.g. "POST /", but only a "GET /" was found.
	// If the http.Handler instance is nil AND the flag is false, the route was not found.
	Resolve(method Method, path string) (http.Handler, bool)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type treeNode struct {
	pathFragment string
	allowed      Method // if different handlers are configured for GET vs POST, etc. add another child node
	handler      http.Handler
	fixed        []*treeNode
	variable     *treeNode
	wildcard     *treeNode
	parent       *treeNode
}

func newTreeNode(path string, allowed Method, handler http.Handler) *treeNode {
	treeNode{
		pathFragment: path,
		allowed:      allowed,
		handler:      handler,
		fixed:        nil,
		variable:     nil,
		wildcard:     nil,
		parent:       nil,
	}
	return &treeNode{}
}

func (this *treeNode) Add(route Route) error {
	// this.allowed = route.AllowedMethods
	pathFragments := strings.Split(route.Path, "/")
	for i, fragment := range pathFragments {
		if i != len(pathFragments)-1 {
			fragment = fragment + "/"
			//parent := pathFragments[i - 1] fixme: find a way to set the parent treenode
		}
		newTreeNode(fragment, route.AllowedMethods, route.Handler)
	}

	this.handler = route.Handler
	return nil
}

func (this *treeNode) Resolve(desired Method, path string) (http.Handler, bool) {
	return this.handler, true
	return nil, false
}
