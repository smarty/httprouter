package httprouter

import (
	"net/http"
	"strings"
	"unicode"
)

type treeNode struct {
	pathFragment string
	static       []*treeNode
	variable     *treeNode
	wildcard     *treeNode
	handlers     *methodHandlers
}

func (this *treeNode) Add(route Route) error {
	if route.AllowedMethods == MethodNone {
		return ErrUnknownMethod
	}

	if len(route.Path) == 0 {
		if this.handlers == nil {
			this.handlers = &methodHandlers{}
		}

		return this.handlers.Add(route.AllowedMethods, route.Handler)
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
	} else if strings.HasPrefix(pathFragmentForChildNode, "*") {
		return this.addWildcard(route, pathFragmentForChildNode)
	} else if strings.HasPrefix(pathFragmentForChildNode, ":") {
		return this.addVariable(route, pathFragmentForChildNode)
	} else {
		return this.addStatic(route, pathFragmentForChildNode)
	}
}
func (this *treeNode) addWildcard(route Route, pathFragment string) error {
	if this.wildcard == nil {
		this.wildcard = &treeNode{pathFragment: pathFragment}
	}

	if len(route.Path) > 1 {
		return ErrInvalidWildcard // must only be "*"
	}

	route.Path = ""
	return this.wildcard.Add(route)
}
func (this *treeNode) addVariable(route Route, pathFragment string) error {
	if this.variable == nil {
		this.variable = &treeNode{pathFragment: pathFragment}
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

	staticChild := &treeNode{pathFragment: pathFragment}
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

func (this *treeNode) Resolve(method, incomingPath string) (http.Handler, bool) {
	if len(incomingPath) == 0 {
		if this.handlers == nil {
			return nil, false
		} else {
			return this.handlers.Resolve(method), true
		}
	}

	if incomingPath[0] == '/' {
		incomingPath = incomingPath[1:]
	}

	var handler http.Handler
	var staticResourceExists, variableResourceExists bool

	var pathFragment = parsePathFragment(incomingPath)
	for _, staticChild := range this.static {
		if pathFragment != staticChild.pathFragment {
			continue
		}

		// the path fragment DOES match...
		remainingPath := incomingPath[len(staticChild.pathFragment):]
		if handler, staticResourceExists = staticChild.Resolve(method, remainingPath); handler != nil {
			return handler, staticResourceExists
		}

		break // don't bother checking any more of siblings of the static child, they don't match
	}

	if this.variable != nil {
		remainingPath := incomingPath[len(pathFragment):]
		if handler, variableResourceExists = this.variable.Resolve(method, remainingPath); handler != nil {
			return handler, variableResourceExists
		}
	}

	if this.wildcard != nil {
		return this.wildcard.Resolve(method, "")
	}

	return nil, staticResourceExists || variableResourceExists
}
func parsePathFragment(value string) string {
	if index := strings.Index(value, "/"); index == -1 {
		return value
	} else {
		return value[0:index]
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type methodHandlers struct {
	Get     http.Handler
	Head    http.Handler
	Post    http.Handler
	Put     http.Handler
	Delete  http.Handler
	Connect http.Handler
	Options http.Handler
	Trace   http.Handler
	Patch   http.Handler
}

func (this *methodHandlers) Add(allowed Method, handler http.Handler) error {
	if allowed&MethodGet == MethodGet && this.Get != nil {
		return ErrRouteExists
	} else if allowed&MethodGet == MethodGet {
		this.Get = handler
	}

	if allowed&MethodHead == MethodHead && this.Head != nil {
		return ErrRouteExists
	} else if allowed&MethodHead == MethodHead {
		this.Head = handler
	}

	if allowed&MethodPost == MethodPost && this.Post != nil {
		return ErrRouteExists
	} else if allowed&MethodPost == MethodPost {
		this.Post = handler
	}

	if allowed&MethodPut == MethodPut && this.Put != nil {
		return ErrRouteExists
	} else if allowed&MethodPut == MethodPut {
		this.Put = handler
	}

	if allowed&MethodDelete == MethodDelete && this.Delete != nil {
		return ErrRouteExists
	} else if allowed&MethodDelete == MethodDelete {
		this.Delete = handler
	}

	if allowed&MethodConnect == MethodConnect && this.Connect != nil {
		return ErrRouteExists
	} else if allowed&MethodConnect == MethodConnect {
		this.Connect = handler
	}

	if allowed&MethodOptions == MethodOptions && this.Options != nil {
		return ErrRouteExists
	} else if allowed&MethodOptions == MethodOptions {
		this.Options = handler
	}

	if allowed&MethodTrace == MethodTrace && this.Trace != nil {
		return ErrRouteExists
	} else if allowed&MethodTrace == MethodTrace {
		this.Trace = handler
	}

	if allowed&MethodPatch == MethodPatch && this.Patch != nil {
		return ErrRouteExists
	} else if allowed&MethodPatch == MethodPatch {
		this.Patch = handler
	}

	return nil
}
func (this *methodHandlers) Resolve(method string) http.Handler {
	switch method {
	case http.MethodGet:
		return this.Get
	case http.MethodHead:
		return this.Head
	case http.MethodPost:
		return this.Post
	case http.MethodPut:
		return this.Put
	case http.MethodDelete:
		return this.Delete
	case http.MethodConnect:
		return this.Connect
	case http.MethodOptions:
		return this.Options
	case http.MethodTrace:
		return this.Trace
	case http.MethodPatch:
		return this.Patch
	default:
		return nil
	}
}
