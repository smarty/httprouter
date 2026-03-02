package httprouter

import "net/http"

type routeResolver interface {
	// Resolve returns an instance of http.Handler and a bitmask of the methods allowed at the matched path.
	// If the http.Handler instance is not nil, the route was fully resolved and can be invoked.
	// If the http.Handler instance is nil AND allowed > 0, the route was found, but the method isn't compatible (e.g. "POST /", but only a "GET /" was found).
	// If the http.Handler instance is nil AND allowed == 0, the route was not found.
	Resolve(method, path string) (http.Handler, Method)
}

type RecoveryFunc func(response http.ResponseWriter, request *http.Request, recovered any)

type Monitor interface {
	Routed(*http.Request)
	NotFound(*http.Request)
	MethodNotAllowed(*http.Request)
	Recovered(*http.Request, any)
}
