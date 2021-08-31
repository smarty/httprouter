package httprouter

import (
	"net/http"
	"strings"
	"unicode"
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
	static       []*treeNode
	variable     *treeNode
	wildcard     *treeNode
	handlers     map[Method]http.Handler
}

// FUTURE: add a "prune" function to where nodes with a single child are combined
// this would be called after all calls to Add have been completed which would finalize the tree.

func (this *treeNode) Add(route Route) error {
	if len(route.Path) == 0 {
		return this.attachHandler(route.AllowedMethod, route.Handler)
	}

	if route.Path[0] == '/' {
		route.Path = route.Path[1:]
	} else {
		return ErrMalformedPath
	}

	var pathFragmentForChildNode string
	slashIndex := strings.Index(route.Path, "/")
	if slashIndex == 0 {
		return ErrMalformedPath // first character is a slash, that means the URL provided looks something like this: /path/to//document (note the double slash)
	} else if slashIndex == -1 {
		pathFragmentForChildNode = route.Path
	} else {
		pathFragmentForChildNode = route.Path[0:slashIndex]
	}

	if !hasOnlyAllowedCharacters(pathFragmentForChildNode) {
		return ErrInvalidCharacters
	}

	if strings.HasPrefix(pathFragmentForChildNode, "*") {
		return this.addWildcard(route, pathFragmentForChildNode)
	}

	if strings.HasPrefix(pathFragmentForChildNode, ":") {
		return this.addVariable(route, pathFragmentForChildNode)
	}

	return this.addStatic(route, pathFragmentForChildNode)
}
func (this *treeNode) attachHandler(allowed Method, handler http.Handler) error {
	if _, contains := this.handlers[allowed]; contains {
		return ErrRouteExists
	}

	this.handlers[allowed] = handler
	return nil
}
func (this *treeNode) addWildcard(route Route, pathFragment string) error {
	if len(route.Path) > 1 {
		return ErrInvalidWildcard // must only be "*"
	}

	if this.wildcard == nil {
		this.wildcard = &treeNode{pathFragment: pathFragment, handlers: map[Method]http.Handler{}}
	}

	route.Path = ""
	return this.wildcard.Add(route)
}
func (this *treeNode) addVariable(route Route, pathFragment string) error {
	if this.variable == nil {
		this.variable = &treeNode{pathFragment: pathFragment, handlers: map[Method]http.Handler{}}
	}

	route.Path = route.Path[len(pathFragment):]
	return this.variable.Add(route)
}
func (this *treeNode) addStatic(route Route, pathFragment string) (err error) {
	route.Path = route.Path[len(pathFragment):]

	for _, staticChild := range this.static {
		if staticChild.pathFragment == pathFragment {
			return staticChild.Add(route)
		}
	}

	staticChild := &treeNode{pathFragment: pathFragment, handlers: map[Method]http.Handler{}}
	if err = staticChild.Add(route); err != nil {
		return err
	}

	this.static = append(this.static, staticChild)
	return nil
}
func hasOnlyAllowedCharacters(input string) bool {
	for index, value := range input {
		if unicode.IsLetter(value) {
			continue // TODO: ASCII only (a-z A-Z)
		} else if unicode.IsDigit(value) {
			continue // TODO: ASCII only (0-9)
		} else if value == '.' || value == '-' || value == '_' {
			continue
		} else if index == 0 && (value == '*' || value == ':') {
			continue
		}

		return false
	}

	return true
}

func (this *treeNode) Resolve(method Method, incomingPath string) (http.Handler, bool) {
	if len(incomingPath) == 0 {
		return this.handlers[method], len(this.handlers) > 0
	}

	if incomingPath[0] == '/' {
		incomingPath = incomingPath[1:]
	}

	var handler http.Handler
	var resourceExists bool
	var pathFragment = parsePathFragment(incomingPath)
	for _, staticChild := range this.static {
		if pathFragment != staticChild.pathFragment {
			continue
		}

		// the path fragment DOES match...
		remainingPath := incomingPath[len(staticChild.pathFragment):]
		if handler, resourceExists = staticChild.Resolve(method, remainingPath); handler != nil {
			return handler, resourceExists
		}

		break // don't bother checking any more of siblings of the static child, they don't match
	}

	if this.variable != nil {
		if strings.HasPrefix(incomingPath, this.variable.pathFragment) {
			remainingPath := incomingPath[len(this.variable.pathFragment):]
			if handler, resourceExists = this.variable.Resolve(method, remainingPath); handler != nil {
				return handler, resourceExists
			}
		}
	}

	if this.wildcard != nil {
		if strings.HasPrefix(incomingPath, this.wildcard.pathFragment) {
			return this.wildcard.Resolve(method, "") // wildcard matches everything, don't bother with the path
		}
	}

	return nil, resourceExists
}
func parsePathFragment(value string) string {
	if index := strings.Index(value, "/"); index == -1 {
		return value
	} else {
		return value[0:index]
	}
}
