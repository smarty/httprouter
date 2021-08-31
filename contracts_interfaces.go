package httprouter

import "net/http"

type Resolver interface {
	Resolve(method, path string) http.Handler
}

type routeResolver interface {
	// Resolve returns an instance of http.Handler and with a flag indicating if the route was understood.
	// If the http.Handler instance is not nil, the route was fully resolved and can be invoked.
	// If the http.Handler instance is nil AND the flag is true, the route was found, but the method isn't compatible (e.g. "POST /", but only a "GET /" was found).
	// If the http.Handler instance is nil AND the flag is false, the route was not found.
	Resolve(method Method, path string) (http.Handler, bool)
}

type RecoveryFunc func(response http.ResponseWriter, request *http.Request, recovered interface{})

type Monitor interface {
	Routed(*http.Request)
	NotFound(*http.Request)
	MethodNotAllowed(*http.Request)
	Recovered(*http.Request, interface{})
}
