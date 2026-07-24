package httprouter

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
)

// These fuzz targets guard the trie's structural optimizations — the compaction pass (tree.go absorbChain/compact)
// and the wide-node static index — against silently changing routing behavior. Rather than diff two router
// implementations (there is only one), they diff the production trie against an independent, deliberately naive
// segment-based oracle: a flat list of patterns matched one at a time with no shared nodes, no compaction, and no
// indexing. Any input where the trie and the oracle disagree is either a trie bug or an oracle bug, and every such
// disagreement is worth investigating.
//
// Run deeply with:  go test -run=x -fuzz=FuzzRouterMatchesOracle
//                    go test -run=x -fuzz=FuzzRouterRobustness
// The seed corpus also executes under `make test`, so the targets double as fast regression tests.

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// segKind labels a single path segment in the oracle's structured pattern. Ordering matters: it is exactly the
// trie's match precedence (static beats variable beats wildcard), so it doubles as the precedence key.
type segKind uint8

const (
	kindStatic segKind = iota
	kindVariable
	kindWildcard
)

type patternSegment struct {
	kind    segKind
	literal string // meaningful only for kindStatic
}

// oraclePattern is one accepted route expressed structurally, independent of the trie.
type oraclePattern struct {
	segments      []patternSegment
	trailingSlash bool
	methods       Method
	handlerID     string
}

// staticVocabulary is intentionally tiny so generated static segments, variables, and query paths collide often —
// collisions are what exercise precedence and the combined-405 union.
var staticVocabulary = []string{"a", "b", "c"}

// the nine supported request methods, paired bit-to-string for the oracle.
var (
	fuzzMethodBits = []Method{
		MethodGet, MethodHead, MethodPost, MethodPut, MethodDelete,
		MethodConnect, MethodOptions, MethodTrace, MethodPatch,
	}
	fuzzMethodStrings = []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete,
		http.MethodConnect, http.MethodOptions, http.MethodTrace, http.MethodPatch,
	}
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// byteStream doles out a fuzz []byte as a sequence of bounded choices, reading zeros once exhausted so short inputs
// still decode deterministically.
type byteStream struct {
	data []byte
	pos  int
}

func (this *byteStream) next() byte {
	if this.pos >= len(this.data) {
		return 0
	}
	value := this.data[this.pos]
	this.pos++
	return value
}
func (this *byteStream) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(this.next()) % n
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func FuzzRouterMatchesOracle(f *testing.F) {
	if testing.Short() {
		f.Skip("fuzz targets (and their seed corpus) run in long mode only")
	}

	f.Add([]byte{})
	f.Add([]byte{1, 0, 2, 0, 0, 0, 0, 0})
	f.Add([]byte{3, 2, 3, 0, 1, 2, 0, 0, 4, 5, 6, 7, 8, 9, 10})
	f.Add([]byte{5, 255, 255, 1, 2, 2, 0, 1, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		stream := &byteStream{data: data}
		tree, patterns := buildTreeAndOracle(stream)
		router := newRouter(tree, notFoundStub{}, methodNotAllowedStub{}, &nop{})

		method, target, querySegments, queryTrailing := decodeCanonicalQuery(stream)

		expectedStatus, expectedBody, expectedAllow := oracleResult(patterns, method, querySegments, queryTrailing)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, &http.Request{Method: method, RequestURI: target, URL: &url.URL{Path: target}})

		if recorder.Code != expectedStatus {
			t.Fatalf("status mismatch for %s %q\nexpected %d, actual %d\nroutes:\n%s",
				method, target, expectedStatus, recorder.Code, describePatterns(patterns))
		}
		if expectedStatus == http.StatusOK && recorder.Body.String() != expectedBody {
			t.Fatalf("body mismatch for %s %q\nexpected %q, actual %q\nroutes:\n%s",
				method, target, expectedBody, recorder.Body.String(), describePatterns(patterns))
		}
		if expectedStatus == http.StatusMethodNotAllowed {
			if actualAllow := recorder.Header().Get("Allow"); actualAllow != expectedAllow {
				t.Fatalf("Allow mismatch for %s %q\nexpected %q, actual %q\nroutes:\n%s",
					method, target, expectedAllow, actualAllow, describePatterns(patterns))
			}
		}
	})
}

// FuzzRouterRobustness throws structured routes plus an ARBITRARY raw method and path at the router, asserting only
// invariants that must hold for any input: it never panics, and its three outcomes stay mutually coherent (a served
// handler reports 200; a 405 always carries a parseable Allow header; a 404 carries none).
func FuzzRouterRobustness(f *testing.F) {
	if testing.Short() {
		f.Skip("fuzz targets (and their seed corpus) run in long mode only")
	}

	f.Add([]byte{2, 0, 1, 0}, "GET", "/a")
	f.Add([]byte{3, 2, 2, 0, 1, 0}, "POST", "/a//b/")
	f.Add([]byte{1, 0, 1, 2}, "", "")

	f.Fuzz(func(t *testing.T, data []byte, method, rawPath string) {
		stream := &byteStream{data: data}
		tree, _ := buildTreeAndOracle(stream)
		router := newRouter(tree, notFoundStub{}, methodNotAllowedStub{}, &nop{})

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, &http.Request{Method: method, RequestURI: rawPath, URL: &url.URL{Path: rawPath}})

		switch recorder.Code {
		case http.StatusOK, http.StatusNotFound:
			if allow := recorder.Header().Get("Allow"); allow != "" {
				t.Fatalf("status %d must not set Allow, got %q", recorder.Code, allow)
			}
		case http.StatusMethodNotAllowed:
			allow := recorder.Header().Get("Allow")
			if allow == "" {
				t.Fatal("405 must set a non-empty Allow header")
			}
			if ParseMethods(strings.ReplaceAll(allow, ", ", "|")) == MethodNone {
				t.Fatalf("405 Allow header %q did not parse to any known method", allow)
			}
		default:
			t.Fatalf("unexpected status %d", recorder.Code)
		}
	})
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// buildTreeAndOracle decodes a set of candidate routes, registers each in a real treeNode, and records the oracle
// pattern ONLY for routes the tree actually accepted — so registration semantics (dedup, invalid wildcards) never
// need to be reimplemented in the oracle. The tree is compacted exactly as New does.
func buildTreeAndOracle(stream *byteStream) (*treeNode, []oraclePattern) {
	const maxRoutes = 6
	const maxSegments = 4

	tree := &treeNode{}
	var patterns []oraclePattern

	routeCount := stream.intn(maxRoutes + 1)
	for routeIndex := 0; routeIndex < routeCount; routeIndex++ {
		methods := decodeMethods(stream)
		segments := decodePatternSegments(stream, maxSegments)
		trailingSlash := lastKind(segments) != kindWildcard && stream.intn(2) == 1

		handlerID := "R" + string(rune('0'+routeIndex))
		route := Route{
			AllowedMethods: methods,
			Path:           patternPath(segments, trailingSlash),
			Handler:        simpleHandler(handlerID),
		}

		if tree.Add(route) != nil {
			continue // rejected (duplicate/invalid) — oracle mirrors by not recording it
		}
		patterns = append(patterns, oraclePattern{
			segments:      segments,
			trailingSlash: trailingSlash,
			methods:       methods,
			handlerID:     handlerID,
		})
	}

	tree.compact()
	return tree, patterns
}

func decodeMethods(stream *byteStream) Method {
	// two bytes so bits 8-9 (TRACE, PATCH) are reachable, then masked to the supported set.
	mask := Method(uint16(stream.next())|uint16(stream.next())<<8) & supportedMethods
	if mask == MethodNone {
		mask = MethodGet
	}
	return mask
}

func decodePatternSegments(stream *byteStream, maxSegments int) []patternSegment {
	segmentCount := stream.intn(maxSegments) + 1 // at least one; the root "/" route works but is out of the segment model's scope (unit-tested elsewhere)
	segments := make([]patternSegment, 0, segmentCount)
	for position := 0; position < segmentCount; position++ {
		lastPosition := position == segmentCount-1
		switch stream.intn(3) {
		case 0:
			segments = append(segments, patternSegment{kind: kindStatic, literal: staticVocabulary[stream.intn(len(staticVocabulary))]})
		case 1:
			segments = append(segments, patternSegment{kind: kindVariable})
		default:
			if lastPosition {
				segments = append(segments, patternSegment{kind: kindWildcard})
			} else {
				// wildcard is only legal as the terminal segment; demote to static elsewhere.
				segments = append(segments, patternSegment{kind: kindStatic, literal: staticVocabulary[stream.intn(len(staticVocabulary))]})
			}
		}
	}
	return segments
}

func lastKind(segments []patternSegment) segKind {
	if len(segments) == 0 {
		return kindStatic
	}
	return segments[len(segments)-1].kind
}

func patternPath(segments []patternSegment, trailingSlash bool) string {
	var builder strings.Builder
	for _, segment := range segments {
		_ = builder.WriteByte('/')
		switch segment.kind {
		case kindStatic:
			_, _ = builder.WriteString(segment.literal)
		case kindVariable:
			_, _ = builder.WriteString(":v")
		case kindWildcard:
			_ = builder.WriteByte('*')
		}
	}
	if trailingSlash {
		_ = builder.WriteByte('/')
	}
	return builder.String()
}

// decodeCanonicalQuery produces a well-formed request: a known method and a path of one or more non-empty segments
// drawn from the same vocabulary, optionally with a single trailing slash. Staying canonical is what lets the oracle
// predict an exact result; adversarial paths are the robustness target's job.
func decodeCanonicalQuery(stream *byteStream) (method, target string, segments []string, trailingSlash bool) {
	method = fuzzMethodStrings[stream.intn(len(fuzzMethodStrings))]

	const maxSegments = 4
	segmentCount := stream.intn(maxSegments) + 1
	segments = make([]string, 0, segmentCount)
	for position := 0; position < segmentCount; position++ {
		segments = append(segments, staticVocabulary[stream.intn(len(staticVocabulary))])
	}
	trailingSlash = stream.intn(2) == 1

	target = "/" + strings.Join(segments, "/")
	if trailingSlash {
		target += "/"
	}
	return method, target, segments, trailingSlash
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// oracleResult predicts the router's observable outcome for a canonical query by matching each pattern independently.
func oracleResult(patterns []oraclePattern, method string, querySegments []string, queryTrailing bool) (status int, body, allow string) {
	requestedBit := methodBit(method)

	var matches []oraclePattern
	for _, pattern := range patterns {
		if patternMatchesQuery(pattern, querySegments, queryTrailing) {
			matches = append(matches, pattern)
		}
	}
	if len(matches) == 0 {
		return http.StatusNotFound, "", ""
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return lessPrecedence(matches[i].segments, matches[j].segments)
	})

	var allowed Method
	for _, pattern := range matches {
		allowed |= pattern.methods
		if pattern.methods&requestedBit != 0 && status != http.StatusOK {
			status, body = http.StatusOK, pattern.handlerID
		}
	}
	if status == http.StatusOK {
		return status, body, ""
	}
	return http.StatusMethodNotAllowed, "", allowed.HeaderValue()
}

// patternMatchesQuery mirrors the trie's match rules at the segment level: static equals, variable consumes exactly
// one (always non-empty in canonical queries), wildcard is terminal and absorbs one-or-more remaining segments (or a
// bare trailing slash), and a non-wildcard pattern must consume the query exactly, trailing slash included.
func patternMatchesQuery(pattern oraclePattern, querySegments []string, queryTrailing bool) bool {
	for position, segment := range pattern.segments {
		if segment.kind == kindWildcard {
			remaining := len(querySegments) - position
			return remaining >= 1 || (remaining == 0 && queryTrailing)
		}
		if position >= len(querySegments) {
			return false
		}
		if segment.kind == kindStatic && querySegments[position] != segment.literal {
			return false
		}
		// kindVariable matches any single non-empty segment; canonical query segments are always non-empty.
	}
	if len(querySegments) != len(pattern.segments) {
		return false // leftover query segments and no wildcard to absorb them
	}
	return pattern.trailingSlash == queryTrailing
}

// lessPrecedence reports whether pattern a outranks pattern b, comparing segment kinds left to right
// (static < variable < wildcard). Co-matching patterns always differ before both run out, so ties don't occur in
// practice; the length fallback is defensive.
func lessPrecedence(a, b []patternSegment) bool {
	for position := 0; position < len(a) && position < len(b); position++ {
		if a[position].kind != b[position].kind {
			return a[position].kind < b[position].kind
		}
	}
	return len(a) < len(b)
}

func methodBit(method string) Method {
	for index, candidate := range fuzzMethodStrings {
		if candidate == method {
			return fuzzMethodBits[index]
		}
	}
	return MethodNone
}

func describePatterns(patterns []oraclePattern) string {
	var builder strings.Builder
	for _, pattern := range patterns {
		_, _ = builder.WriteString("  " + pattern.methods.String() + " " + patternPath(pattern.segments, pattern.trailingSlash) + " -> " + pattern.handlerID + "\n")
	}
	return builder.String()
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type notFoundStub struct{}

func (notFoundStub) ServeHTTP(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(404)
}

type methodNotAllowedStub struct{}

func (methodNotAllowedStub) ServeHTTP(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(405)
}
