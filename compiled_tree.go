package httprouter

import (
	"net/http"
	"strings"
)

// compiledTree is the immutable, request-time representation of a route tree. Nodes and static edges live in
// contiguous arenas so resolving a route does not have to chase separately allocated treeNode pointers.
type compiledTree struct {
	nodes       []compiledNode
	staticEdges []compiledEdge
	wideIndexes []map[string]uint32
	handlers    []methodHandlers
}

// compiledNode links only into the compiledTree arenas. Keeping request-time nodes together avoids the scattered
// allocations of the registration tree; path fragments live on their incoming static edges.
type compiledNode struct {
	staticStart uint32
	staticCount uint32
	wideIndex   uint32
	variable    *compiledNode
	wildcard    *compiledNode
	handlers    *methodHandlers
}

type compiledEdge struct {
	pathFragment string
	child        *compiledNode
}

const noCompiledIndex = ^uint32(0)

// compileTree freezes the mutable registration tree into compact request-time arenas. The source tree must have
// completed its compaction pass before this function is called.
func compileTree(root *treeNode) *compiledTree {
	compiled := &compiledTree{}
	nodeIndexes := make(map[*treeNode]uint32)
	var sourceNodes []*treeNode

	var assignNodeIndexes func(*treeNode)
	assignNodeIndexes = func(source *treeNode) {
		nodeIndexes[source] = uint32(len(sourceNodes))
		sourceNodes = append(sourceNodes, source)

		for _, child := range source.static {
			assignNodeIndexes(child)
		}
		if source.variable != nil {
			assignNodeIndexes(source.variable)
		}
		if source.wildcard != nil {
			assignNodeIndexes(source.wildcard)
		}
	}
	assignNodeIndexes(root)

	compiled.nodes = make([]compiledNode, len(sourceNodes))
	staticEdgeCount := 0
	handlerCount := 0
	for _, source := range sourceNodes {
		staticEdgeCount += len(source.static)
		if source.handlers != nil {
			handlerCount++
		}
	}
	compiled.staticEdges = make([]compiledEdge, 0, staticEdgeCount)
	compiled.handlers = make([]methodHandlers, handlerCount)

	nextHandler := 0
	for nodeIndex, source := range sourceNodes {
		target := &compiled.nodes[nodeIndex]
		target.staticStart = uint32(len(compiled.staticEdges))
		target.staticCount = uint32(len(source.static))
		target.wideIndex = noCompiledIndex

		for _, child := range source.static {
			compiled.staticEdges = append(compiled.staticEdges, compiledEdge{
				pathFragment: child.pathFragment,
				child:        &compiled.nodes[nodeIndexes[child]],
			})
		}

		if len(source.static) >= staticIndexThreshold {
			index := make(map[string]uint32, len(source.static))
			for offset, child := range source.static {
				firstSegment := child.pathFragment
				if slash := strings.IndexByte(firstSegment, '/'); slash >= 0 {
					firstSegment = firstSegment[:slash]
				}
				index[firstSegment] = target.staticStart + uint32(offset)
			}
			target.wideIndex = uint32(len(compiled.wideIndexes))
			compiled.wideIndexes = append(compiled.wideIndexes, index)
		}

		if source.variable != nil {
			target.variable = &compiled.nodes[nodeIndexes[source.variable]]
		}
		if source.wildcard != nil {
			target.wildcard = &compiled.nodes[nodeIndexes[source.wildcard]]
		}
		if source.handlers != nil {
			compiled.handlers[nextHandler] = *source.handlers
			target.handlers = &compiled.handlers[nextHandler]
			nextHandler++
		}
	}

	return compiled
}

func (this *compiledTree) Resolve(method, incomingPath string) (http.Handler, Method) {
	return this.resolve(&this.nodes[0], method, incomingPath)
}

// resolve loops across deterministic edges and recurses only when the current node has a fallback route that
// must be tried if a higher-priority static or variable route does not resolve the requested method.
func (this *compiledTree) resolve(node *compiledNode, method, incomingPath string) (http.Handler, Method) {
	var suppressAllowedOnSuccess bool

	for {
		if len(incomingPath) == 0 {
			if node.handlers == nil {
				return nil, 0
			}
			handlers := node.handlers
			handler := handlers.Resolve(method)
			if handler != nil && suppressAllowedOnSuccess {
				return handler, MethodNone
			}
			return handler, handlers.allowed
		}

		if incomingPath[0] == '/' {
			incomingPath = incomingPath[1:]
		}

		var staticEdge compiledEdge
		var staticMatched bool

		if node.wideIndex != noCompiledIndex {
			firstSegment := incomingPath
			if slash := strings.IndexByte(firstSegment, '/'); slash >= 0 {
				firstSegment = firstSegment[:slash]
			}

			if edgeIndex, found := this.wideIndexes[node.wideIndex][firstSegment]; found {
				candidate := this.staticEdges[edgeIndex]
				fragmentLength := len(candidate.pathFragment)
				if fragmentLength == len(firstSegment) ||
					len(incomingPath) >= fragmentLength &&
						incomingPath[:fragmentLength] == candidate.pathFragment &&
						(fragmentLength == len(incomingPath) || incomingPath[fragmentLength] == '/') {
					staticEdge = candidate
					staticMatched = true
				}
			}
		} else {
			staticEnd := node.staticStart + node.staticCount
			for edgeIndex := node.staticStart; edgeIndex < staticEnd; edgeIndex++ {
				candidate := this.staticEdges[edgeIndex]
				fragmentLength := len(candidate.pathFragment)
				if len(incomingPath) < fragmentLength {
					continue
				}
				if fragmentLength > 0 && incomingPath[0] != candidate.pathFragment[0] {
					continue
				}
				if incomingPath[:fragmentLength] != candidate.pathFragment {
					continue
				}
				if fragmentLength != len(incomingPath) && incomingPath[fragmentLength] != '/' {
					continue
				}

				staticEdge = candidate
				staticMatched = true
				break
			}
		}

		var staticAllowed Method
		if staticMatched {
			remainingPath := incomingPath[len(staticEdge.pathFragment):]
			if node.variable == nil && node.wildcard == nil {
				node = staticEdge.child
				incomingPath = remainingPath
				suppressAllowedOnSuccess = true
				continue
			}

			if handler, allowed := this.resolve(staticEdge.child, method, remainingPath); handler != nil {
				return handler, MethodNone
			} else {
				staticAllowed = allowed
			}
		}

		var variableAllowed Method
		if node.variable != nil {
			var remainingPath string
			if slash := strings.IndexByte(incomingPath, '/'); slash >= 0 {
				remainingPath = incomingPath[slash:]
			}

			if !staticMatched && node.wildcard == nil {
				node = node.variable
				incomingPath = remainingPath
				suppressAllowedOnSuccess = true
				continue
			}

			if handler, allowed := this.resolve(node.variable, method, remainingPath); handler != nil {
				return handler, MethodNone
			} else {
				variableAllowed = allowed
			}
		}

		if node.wildcard != nil {
			if !staticMatched && node.variable == nil {
				node = node.wildcard
				incomingPath = ""
				continue
			}

			handler, wildcardAllowed := this.resolve(node.wildcard, method, "")
			return handler, staticAllowed | variableAllowed | wildcardAllowed
		}

		return nil, staticAllowed | variableAllowed
	}
}
