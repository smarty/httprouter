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
	if route.AllowedMethods == 0 ||
		route.AllowedMethods&MethodNone != 0 ||
		route.AllowedMethods&^supportedMethods != 0 {
		return ErrUnknownMethod
	}
	if route.Handler == nil {
		return ErrNilHandler
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

// compact collapses chains of single-child static nodes into multi-segment fragments, turning the per-segment
// trie into a compressed radix tree. It is a one-time pass run after all routes are registered — the tree is
// immutable thereafter — so no edge-splitting is ever needed. It descends the whole tree but never extends a
// node's own fragment; only a node's static children (genuine matched literals) are absorbed, via absorbChain.
// This is why it is safe to call on the root, and on variable/wildcard nodes, whose own fragments are consumed
// positionally rather than matched literally and so must never grow.
func (this *treeNode) compact() {
	for _, staticChild := range this.static {
		staticChild.absorbChain()
		staticChild.compact()
	}
	if this.variable != nil {
		this.variable.compact()
	}
	if this.wildcard != nil {
		this.wildcard.compact()
	}
}

// absorbChain extends this static literal node by swallowing its lone static child for as long as the chain
// stays unambiguous — exactly one static child, no variable or wildcard sibling, and no handler terminating
// here. Merging appends "/<childFragment>" to this node's fragment and adopts the child's children, static
// index, and handlers. The node pointer is unchanged, so the parent needs no rewiring, and a wide node's
// staticIndex keys stay valid: absorption only ever extends fragments, never alters a node's first segment (the
// map key) or its identity. The empty-fragment trailing-slash leaf is left alone — absorbing it would only
// splice a '/' onto the fragment for no benefit.
func (this *treeNode) absorbChain() {
	for len(this.static) == 1 && this.variable == nil && this.wildcard == nil && this.handlers == nil {
		onlyChild := this.static[0]
		if onlyChild.pathFragment == "" {
			return
		}
		this.pathFragment += "/" + onlyChild.pathFragment
		this.static = onlyChild.static
		this.staticIndex = onlyChild.staticIndex
		this.variable = onlyChild.variable
		this.wildcard = onlyChild.wildcard
		this.handlers = onlyChild.handlers
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

// Resolve walks the tree iteratively. Whenever a node's only viable continuation is a single deterministic edge —
// a static match with no variable or wildcard sibling to fall back to, or a variable with no static match and no
// wildcard — the walk reassigns the receiver and loops instead of recursing, so a non-branching path costs no
// stack frames at all. Recursion is kept only where a node genuinely has an alternative to try if the
// higher-priority edge fails to resolve the requested method.
func (this *treeNode) Resolve(method, incomingPath string) (http.Handler, Method) {
	for {
		if len(incomingPath) == 0 {
			if this.handlers == nil {
				return nil, 0
			}
			return this.handlers.Resolve(method), this.handlers.allowed
		}

		if incomingPath[0] == '/' {
			incomingPath = incomingPath[1:]
		}

		var matchedStatic *treeNode

		if len(this.static) >= staticIndexThreshold {
			// Wide node: probe the map by the incoming path's first segment (the map key), then match the
			// candidate child's full fragment — which may span several segments once the compaction pass has
			// merged a single-child chain into it.
			var firstSegment string
			if slash := strings.IndexByte(incomingPath, '/'); slash < 0 {
				firstSegment = incomingPath
			} else {
				firstSegment = incomingPath[:slash]
			}
			if staticChild, found := this.staticIndex[firstSegment]; found {
				fragmentLength := len(staticChild.pathFragment)
				if fragmentLength == len(firstSegment) {
					// Single-segment child (the common case): the map hit on firstSegment already fully
					// validated the match and the boundary — no re-comparison needed.
					matchedStatic = staticChild
				} else if len(incomingPath) >= fragmentLength && incomingPath[:fragmentLength] == staticChild.pathFragment &&
					(fragmentLength == len(incomingPath) || incomingPath[fragmentLength] == '/') {
					// Multi-segment (compacted) child: the map matched only the first segment, so confirm the
					// rest of the fused fragment and its trailing segment boundary before descending.
					matchedStatic = staticChild
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

				matchedStatic = staticChild
				break
			}
		}

		var staticAllowed, variableAllowed Method

		if matchedStatic != nil {
			remainingPath := incomingPath[len(matchedStatic.pathFragment):]
			if this.variable == nil && this.wildcard == nil {
				this = matchedStatic
				incomingPath = remainingPath
				continue
			}
			handler, allowed := matchedStatic.Resolve(method, remainingPath)
			if handler != nil {
				return handler, MethodNone
			}
			staticAllowed = allowed
		}

		if this.variable != nil && len(incomingPath) > 0 && incomingPath[0] != '/' {
			// A variable consumes exactly one segment, so the boundary is needed here.
			var remainingPath string
			if slash := strings.IndexByte(incomingPath, '/'); slash >= 0 {
				remainingPath = incomingPath[slash:]
			}
			if matchedStatic == nil && this.wildcard == nil {
				this = this.variable
				incomingPath = remainingPath
				continue
			}
			handler, allowed := this.variable.Resolve(method, remainingPath)
			if handler != nil {
				return handler, MethodNone
			}
			variableAllowed = allowed
		}

		if this.wildcard != nil {
			if matchedStatic == nil && this.variable == nil {
				this = this.wildcard
				incomingPath = ""
				continue
			}
			handler, wildcardAllowed := this.wildcard.Resolve(method, "")
			return handler, staticAllowed | variableAllowed | wildcardAllowed
		}

		return nil, staticAllowed | variableAllowed
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
	if this.allowed&allowed&^MethodNone != 0 {
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
