package httprouter

import (
	"net/http"
	"strings"
)

type Route struct {
	AllowedMethod Method
	Path           string
	Handler        http.Handler
}

func ParseRoutes(allowedMethods string, paths string, handler http.Handler) (routes []Route) {
	paths = strings.TrimSpace(paths)

	for _, item := range strings.Split(paths, pipeDelimiter) {
		routes = append(routes, ParseRoute(allowedMethods, item, handler))
	}

	return routes
}
func ParseRoute(allowedMethods string, path string, handler http.Handler) Route {
	return Route{
		AllowedMethod: ParseMethods(allowedMethods),
		Path:           strings.TrimSpace(path),
		Handler:        handler,
	}
}
func (this Route) String() string   { return this.AllowedMethod.String() + " " + this.Path }
func (this Route) GoString() string { return this.String() }

const pipeDelimiter = "|"
