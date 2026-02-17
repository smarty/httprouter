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
	this.resolve(request).ServeHTTP(response, request)
}
func (this *defaultRouter) resolve(request *http.Request) http.Handler {
	rawPath := request.RequestURI
	if len(rawPath) == 0 {
		rawPath = request.URL.Path
	} else if index := strings.IndexByte(rawPath, '?'); index >= 0 {
		rawPath = rawPath[0:index]
	}

	if handler, resolved := this.resolver.Resolve(request.Method, rawPath); handler != nil {
		this.monitor.Routed(request)
		return handler
	} else if resolved {
		this.monitor.MethodNotAllowed(request)
		return this.methodNotAllowed
	} else {
		this.monitor.NotFound(request)
		return this.notFound
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
