package httprouter

import "net/http"

type Resolver interface {
	Resolve(method, path string) http.Handler
}

type RecoveryFunc func(response http.ResponseWriter, request *http.Request, recovered interface{})

type Monitor interface {
	Routed(*http.Request)
	NotFound(*http.Request)
	MethodNotAllowed(*http.Request)
	Recovered(*http.Request, interface{})
}
