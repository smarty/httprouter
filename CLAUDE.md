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

## Testing Patterns

- Custom inline assertion helper: `Assert(t).That(actual).Equals(expected)` and `.IsNil()` — no external test libraries
- `simpleHandler` (string-based `http.Handler` that writes its value as response body) and `assertRoute` helper for integration-style HTTP tests
- Tests use `httptest.NewRequest` and `httptest.NewRecorder`

## File Organization

Files are prefixed `contracts_` for public types, interfaces, errors, config, and method definitions. `router.go` holds the HTTP handler implementations. `tree.go` holds the trie routing algorithm.

Separator comments (`////////...`) are used as horizontal rules between logical sections within files.
