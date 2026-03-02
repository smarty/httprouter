package httprouter

import (
	"net/http"
	"strings"
)

type defaultRouter struct {
	resolver         routeResolver
	notFound         http.Handler
	methodNotAllowed http.Handler
	monitor          Monitor
}

func newRouter(resolver routeResolver, notFound, methodNotAllowed http.Handler, monitor Monitor) http.Handler {
	return &defaultRouter{resolver: resolver, notFound: notFound, methodNotAllowed: methodNotAllowed, monitor: monitor}
}
func (this *defaultRouter) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	rawPath := request.RequestURI
	if len(rawPath) == 0 {
		rawPath = request.URL.Path
	} else if index := strings.IndexByte(rawPath, '?'); index >= 0 {
		rawPath = rawPath[0:index]
	}

	handler, allowed := this.resolver.Resolve(request.Method, rawPath)
	if handler != nil {
		this.monitor.Routed(request)
		handler.ServeHTTP(response, request)
	} else if allowed > 0 {
		this.monitor.MethodNotAllowed(request)
		response.Header().Set("Allow", allowed.HeaderValue())
		this.methodNotAllowed.ServeHTTP(response, request)
	} else {
		this.monitor.NotFound(request)
		this.notFound.ServeHTTP(response, request)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type recoveryRouter struct {
	http.Handler
	recovery RecoveryFunc
	monitor  Monitor
}

func newRecoveryRouter(handler http.Handler, recovery RecoveryFunc, monitor Monitor) *recoveryRouter {
	return &recoveryRouter{Handler: handler, recovery: recovery, monitor: monitor}
}
func (this *recoveryRouter) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	defer func() {
		if recovered := recover(); recovered != nil {
			this.recovery(response, request, recovered)
			this.monitor.Recovered(request, recovered)
		}
	}()

	this.Handler.ServeHTTP(response, request)
}
