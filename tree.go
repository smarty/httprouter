package httprouter

import (
	"net/http"
	"strings"
)

type treeNode struct {
	pathFragment string
	static       []*treeNode
	staticIndex  map[string]*treeNode // populated only once static children exceed staticIndexThreshold; nil for narrow nodes
	variable     *treeNode
	wildcard     *treeNode
	handlers     *methodHandlers
}

// staticIndexThreshold is the number of static children beyond which a node maintains a map for O(1) lookups
// instead of a linear scan. Below it, the linear scan over the slice is faster and more cache-friendly than
// hashing the path fragment, so the map is not built.
const staticIndexThreshold = 8

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
	slashIndex := strings.IndexByte(route.Path, '/')
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
	this.indexStatic(staticChild)
	return nil
}

// indexStatic maintains the staticIndex map for wide nodes. Once the child count crosses staticIndexThreshold the
// map is built (one-time) and thereafter kept in sync as new children are appended. This runs during route
// registration, not on the request hot path.
func (this *treeNode) indexStatic(staticChild *treeNode) {
	if this.staticIndex != nil {
		this.staticIndex[staticChild.pathFragment] = staticChild
		return
	}

	if len(this.static) < staticIndexThreshold {
		return
	}

	this.staticIndex = make(map[string]*treeNode, len(this.static))
	for _, existingChild := range this.static {
		this.staticIndex[existingChild.pathFragment] = existingChild
	}
}
func hasOnlyAllowedCharacters(input string) bool {
	for index, value := range input {
		if _, ok := allowedCharacters[value]; ok {
			continue
		} else if index == 0 && (value == '*' || value == ':') {
			continue
		}

		return false
	}

	return true
}

func (this *treeNode) Resolve(method, incomingPath string) (http.Handler, Method) {
	if len(incomingPath) == 0 {
		if this.handlers == nil {
			return nil, 0
		}
		return this.handlers.Resolve(method), this.handlers.allowed
	}

	if incomingPath[0] == '/' {
		incomingPath = incomingPath[1:]
	}

	var handler http.Handler
	var staticAllowed, variableAllowed Method

	if len(this.static) >= staticIndexThreshold {
		// Wide node: the map is keyed by the segment string, so compute the fragment for the probe.
		var pathFragment string
		if slash := strings.IndexByte(incomingPath, '/'); slash < 0 {
			pathFragment = incomingPath
		} else {
			pathFragment = incomingPath[:slash]
		}
		if staticChild, found := this.staticIndex[pathFragment]; found {
			if handler, staticAllowed = staticChild.Resolve(method, incomingPath[len(pathFragment):]); handler != nil {
				return handler, MethodNone
			}
		}
	} else {
		// Narrow node: match each child fragment as a segment prefix — no IndexByte scan needed.
		for _, staticChild := range this.static {
			fragmentLength := len(staticChild.pathFragment)
			if len(incomingPath) < fragmentLength {
				continue
			}
			if fragmentLength > 0 && incomingPath[0] != staticChild.pathFragment[0] {
				continue // cheap first-byte reject before the full compare (empty fragment: trailing-slash child)
			}
			if incomingPath[:fragmentLength] != staticChild.pathFragment {
				continue
			}
			if fragmentLength != len(incomingPath) && incomingPath[fragmentLength] != '/' {
				continue // segment boundary mismatch (e.g. child "stuff" vs segment "stuffx")
			}

			if handler, staticAllowed = staticChild.Resolve(method, incomingPath[fragmentLength:]); handler != nil {
				return handler, MethodNone
			}

			break
		}
	}

	if this.variable != nil {
		// A variable consumes exactly one segment, so the boundary is needed here.
		var remainingPath string
		if slash := strings.IndexByte(incomingPath, '/'); slash >= 0 {
			remainingPath = incomingPath[slash:]
		}
		if handler, variableAllowed = this.variable.Resolve(method, remainingPath); handler != nil {
			return handler, MethodNone
		}
	}

	if this.wildcard != nil {
		wildcardHandler, wildcardAllowed := this.wildcard.Resolve(method, "")
		return wildcardHandler, staticAllowed | variableAllowed | wildcardAllowed
	}

	return nil, staticAllowed | variableAllowed
}
func parsePathFragment(value string) string {
	if index := strings.IndexByte(value, '/'); index == -1 {
		return value
	} else {
		return value[0:index]
	}
}

var allowedCharacters = map[rune]struct{}{
	// lower a-z
	'a': {}, 'b': {}, 'c': {}, 'd': {}, 'e': {}, 'f': {}, 'g': {},
	'h': {}, 'i': {}, 'j': {}, 'k': {}, 'l': {}, 'm': {}, 'n': {},
	'o': {}, 'p': {}, 'q': {}, 'r': {}, 's': {}, 't': {}, 'u': {},
	'v': {}, 'w': {}, 'x': {}, 'y': {}, 'z': {},
	// upper A-Z
	'A': {}, 'B': {}, 'C': {}, 'D': {}, 'E': {}, 'F': {}, 'G': {},
	'H': {}, 'I': {}, 'J': {}, 'K': {}, 'L': {}, 'M': {}, 'N': {},
	'O': {}, 'P': {}, 'Q': {}, 'R': {}, 'S': {}, 'T': {}, 'U': {},
	'V': {}, 'W': {}, 'X': {}, 'Y': {}, 'Z': {},
	// 0-9
	'0': {}, '1': {}, '2': {}, '3': {}, '4': {},
	'5': {}, '6': {}, '7': {}, '8': {}, '9': {},
	// other characters
	'.': {}, '-': {}, '_': {},
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type methodHandlers struct {
	allowed Method
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
	if allowed&MethodGet == MethodGet && this.Get != nil ||
		allowed&MethodHead == MethodHead && this.Head != nil ||
		allowed&MethodPost == MethodPost && this.Post != nil ||
		allowed&MethodPut == MethodPut && this.Put != nil ||
		allowed&MethodDelete == MethodDelete && this.Delete != nil ||
		allowed&MethodConnect == MethodConnect && this.Connect != nil ||
		allowed&MethodOptions == MethodOptions && this.Options != nil ||
		allowed&MethodTrace == MethodTrace && this.Trace != nil ||
		allowed&MethodPatch == MethodPatch && this.Patch != nil {
		return ErrRouteExists
	}

	// allow handler to be registered multiple times
	if allowed&MethodGet == MethodGet {
		this.Get = handler
	}
	if allowed&MethodHead == MethodHead {
		this.Head = handler
	}
	if allowed&MethodPost == MethodPost {
		this.Post = handler
	}
	if allowed&MethodPut == MethodPut {
		this.Put = handler
	}
	if allowed&MethodDelete == MethodDelete {
		this.Delete = handler
	}
	if allowed&MethodConnect == MethodConnect {
		this.Connect = handler
	}
	if allowed&MethodOptions == MethodOptions {
		this.Options = handler
	}
	if allowed&MethodTrace == MethodTrace {
		this.Trace = handler
	}
	if allowed&MethodPatch == MethodPatch {
		this.Patch = handler
	}

	this.allowed |= allowed
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
