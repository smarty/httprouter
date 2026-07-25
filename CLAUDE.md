# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

- `make test` — primary development command (runs `go mod tidy`, `go fmt ./...`, then `go test` with `-race -covermode=atomic -timeout=1s -short`)
- `make build` — runs test + compile
- Run a single test: `go test -run TestRouting -v ./...`

## Architecture

This is a zero-dependency HTTP router for Go that implements `http.Handler`. The package is a single flat package (`httprouter`) with no sub-packages.

### Core Components

**Route tree (`tree.go`)** — A trie where each node holds a path fragment and three kinds of children: `static` (exact match), `variable` (`:param`, matches one segment), and `wildcard` (`*`, matches remaining path). Leaf nodes carry a `methodHandlers` struct with one `http.Handler` field per HTTP method, dispatched via switch.

**Router (`router.go`)** — Two `http.Handler` implementations: `defaultRouter` resolves routes and dispatches to the matched handler, not-found (404), or method-not-allowed (405); `recoveryRouter` wraps another handler with `defer/recover`.

**Configuration (`contracts_config.go`)** — Functional options pattern. Public API is `New(options ...Option) (http.Handler, error)` and `RequireNew(options ...Option) http.Handler`. Options are accessed through the `Options` package-level singleton (e.g., `Options.Routes(...)`, `Options.NotFound(...)`).

**Method bitmask (`contracts_methods.go`)** — `Method` is a `uint16` bitmask supporting all 9 standard HTTP methods. Methods are parsed from pipe-delimited strings (e.g., `"GET|POST"`).

### Request Resolution Flow

1. Extract raw path from `request.RequestURI` (falls back to `request.URL.Path`), strip query string
2. Walk the trie: try static children first, then variable, then wildcard
3. Three outcomes: handler found (serve it), route found but wrong method (405), no route (404)

## Performance

### Do not optimize the wildcard branch at the expense of the static branch

`treeNode.Resolve` is the hot path, and its *instruction footprint* is a first-class constraint, not just its
algorithmic step count. Every path through the function pays for the function's total size — including paths that
never execute the code you added.

The rejected optimization: a wildcard node is provably always a childless leaf carrying handlers, because
`addWildcard` rejects any fragment longer than `*` and registers with an empty remaining path, so `Add` can only ever
have populated its handler table. That makes `this.wildcard.Resolve(method, "")` replaceable by reading
`this.wildcard.handlers` directly. It is strictly less work, and it does speed up wildcard resolution — but it places
a `methodHandlers.Resolve` call inside `treeNode.Resolve`, which lets the compiler inline that nine-case string switch
and bloats the function:

| Variant                                    | `treeNode.Resolve` size |
|--------------------------------------------|-------------------------|
| Recursive walk (before `3859698`)          | 1669 bytes              |
| Iterative walk (current)                   | 1651 bytes              |
| Iterative + wildcard read, one call site   | 2088 bytes              |
| Iterative + wildcard read, two call sites  | 2344 bytes              |

Measured cost of the 2088-byte version against the current one: `BenchmarkTreeWildcard` −3.6% and
`BenchmarkRESTWildcard` −14.3%, but `BenchmarkTreeDeepStatic` +9.7%, and `BenchmarkRESTStatic` fell from −14.8% to
−6.8%. Geomean got worse (−2.0% vs −2.9%). Static resolution is the common case; wildcards are not.

Two consequences are therefore accepted deliberately — do not "fix" either in isolation:

- `BenchmarkTreeWildcard` runs ~9% slower than the recursive implementation did. `BenchmarkRESTWildcard` — the
  router-level wildcard case with a realistic route table — is ~11% *faster*, and that is the number that matters.
- `BenchmarkTreeStatic` runs ~4% slower (0.16ns). It resolves `/`, meaning root plus the empty-fragment
  trailing-slash child: the shallowest possible walk, where there are no recursive frames to eliminate and the
  loop's extra live state is pure cost.

Check the size before and after any change to `Resolve`, and keep it near 1651 bytes:

```
go build -gcflags=-S ./ 2>&1 | grep -o 'treeNode).Resolve STEXT size=[0-9]*'
```

### The benchmark noise floor is ±3%

Code layout alone moves these benchmarks. Rebuilding *identical* logic three times with dead padding that shifts
layout but cannot change semantics produced a ±3% spread (`RouterLongStatic` up to +2.75%). Do not act on sub-3%
deltas. Do not compare runs whose binaries contain different sets of test functions either — adding a benchmark file
relocates the others. Use `benchstat` with `-count=10` or more, and measure in the repository itself rather than in a
scratch copy, so both sides of the comparison share a test-file set.

### The compiled arena tree was measured and rejected twice

A second request-time tree holding nodes and static edges in contiguous arenas (`4b5330b`, reverted in `05d1ec0`) is
3.5% *slower* than the pointer tree overall. `compiledNode` still holds pointers for its variable, wildcard, and edge
children, so it never stops chasing them; it only adds slice bounds checks plus a double indirection for wide nodes.
It also could never have been a drop-in replacement: `compileTree` consumes a `*treeNode`, so the registration tree
and its `compact()` pass stay regardless, and the differential fuzzer builds a `*treeNode` — it would no longer cover
the shipped resolver. The ~7-10% originally credited to the arena actually came from the iterative walk bundled into
the same commit, which now lives in `tree.go` with no second tree to maintain.

## Testing Patterns

- Custom inline assertion helper: `Assert(t).That(actual).Equals(expected)` and `.IsNil()` — no external test libraries
- `simpleHandler` (string-based `http.Handler` that writes its value as response body) and `assertRoute` helper for integration-style HTTP tests
- Tests use `httptest.NewRequest` and `httptest.NewRecorder`

## File Organization

Files are prefixed `contracts_` for public types, interfaces, errors, config, and method definitions. `router.go` holds the HTTP handler implementations. `tree.go` holds the trie routing algorithm.

Separator comments (`////////...`) are used as horizontal rules between logical sections within files.
