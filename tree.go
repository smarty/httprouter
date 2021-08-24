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

//type treeNode struct {
//	pathFragment string
//	allowed      Method // if different handlers are configured for GET vs POST, etc. add another child node
//	handler      http.Handler
//	fixed        []*treeNode
//	variable     *treeNode
//	wildcard     *treeNode
//	parent       *treeNode
//}

type treeNode struct {
	pathFragment   string
	staticChildren []*treeNode
	variableChild  *treeNode
	wildcardChild  *treeNode
	handlers       map[Method]http.Handler
}

func main() {
	tree := &treeNode{}
	// tree.Add(routes)

	method := "GET"
	incomingPath := "/path/whatever"
	handler, resourceExists := tree.Resolve(method, incomingPath)
}

func (this *treeNode) Add(route Route) error {
	if len(route.Path) == 0 {
		return nil
	}
	//TODO: get rid of
	handler, pathExists := this.Resolve(route.AllowedMethod, route.Path)
	if handler == route.Handler && pathExists {
		return nil
	} else if handler != route.Handler && pathExists {
		// TODO: if method already exists, should we return some kind of error
		// to indicate that we are overwriting the handler for a particular method?
		this.handlers[route.AllowedMethod] = route.Handler
	}

	slashIndex := strings.Index(route.Path, "/")
	if slashIndex == 0 {
		// first character is a slash, that means the URL provided looks something like this:
		// /path/to//document # note the double slash
		return nil //ErrMalformedRoute
	}

	var pathFragmentForChildNode string

	// -1:   doesn't contain --> it is something like /identity
	if slashIndex == -1 {
		pathFragmentForChildNode = route.Path
	} else {
		pathFragmentForChildNode = route.Path[0 : slashIndex+1] // includes the trailing slash
	}

	// does this incoming route fragement indicate a static, variable, or wildcard child?
	// TODO: (ensure only allowed characters) [A-Z a-z 0-9 _ - . : * ]

	route.Path = route.Path[slashIndex+1:] // TODO: strip off what we've already considered

	if strings.HasPrefix(pathFragmentForChildNode, "*") {
		wildChildRoute := Route{
			AllowedMethod: route.AllowedMethod,
			Path:          route.Path,
			Handler:       route.Handler,
		}
		return this.addWildcardChild(wildChildRoute)
	}

	if strings.HasPrefix(pathFragmentForChildNode, ":") {
		variableChildRoute := Route{
			AllowedMethod: route.AllowedMethod,
			Path:          route.Path,
			Handler:       route.Handler,
		}
		return this.addVariableChild(variableChildRoute)
	}

	staticChildRoute := Route{
		AllowedMethod: route.AllowedMethod,
		Path:          route.Path,
		Handler:       route.Handler,
	}
	return this.addStaticChild(staticChildRoute)
}
func (this *treeNode) addWildcardChild(route Route) error {
	// validate incoming route.Path (must only be "*")
	if len(route.Path) > 1 {
		return nil //errInvalidWildCard //Todo
	}
	route.Path = "" // now truncate it to ""

	if this.wildcardChild != nil {
		// wildcard child already exists, attach a handler for the specific method
		return this.wildcardChild.Add(route)
	}

	this.wildcardChild = &treeNode{handlers: map[Method]http.Handler{}}
	return this.wildcardChild.Add(route)
}
func (this *treeNode) addVariableChild(route Route) error {
	route.Path = route.Path[len(this.pathFragment):]
	//todo: create error checking function
	if this.variableChild != nil {
		return this.variableChild.Add(route)
	}

	this.variableChild = &treeNode{handlers: map[Method]http.Handler{}}
	return this.variableChild.Add(route)
}
func (this *treeNode) addStaticChild(route Route) error {
	route.Path = route.Path[len(this.pathFragment):]

	staticChild := &treeNode{handlers: map[Method]http.Handler{}}
	if err := staticChild.Add(route); err != nil {
		return err
	}

	this.staticChildren = append(this.staticChildren, staticChild)
	return nil
}

func (this *treeNode) Resolve(method Method, incomingPath string) (http.Handler, bool) {
	if len(incomingPath) == 0 {
		//return 405 error if the handler is nil
		return this.handlers[method], true // why true? because we got to a place where the resource exists
	}

	var resourceExists bool
	for _, staticChild := range this.staticChildren {
		if !strings.HasPrefix(incomingPath, staticChild.pathFragment) {
			continue // the child doesn't match, skip it
		}

		// at this point, the path fragment DOES match...
		remainingPath := incomingPath[len(staticChild.pathFragment):]
		handler, resourceExists := staticChild.Resolve(method, remainingPath)
		if handler != nil {
			return handler, resourceExists
		}

		break // don't bother checking any more of sibilings of the static child, they don't match
	}

	if this.variableChild != nil {
		remainingPath := incomingPath[len(this.variableChild.pathFragment):]
		handler, resourceExists := this.variableChild.Resolve(method, remainingPath)
		if handler != nil {
			return handler, resourceExists
		}

	}

	if this.wildcardChild != nil {
		return this.wildcardChild.Resolve(method, "") // wildcard matches everything, don't bother with the path
	}

	//nothing matches -- return 404 error
	return nil, resourceExists // no wildcard children
}
