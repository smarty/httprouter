package httprouter

import (
	"net/http"
	"strings"
)

type Route struct {
	AllowedMethods Method
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
		AllowedMethods: ParseMethods(allowedMethods),
		Path:           strings.TrimSpace(path),
		Handler:        handler,
	}
}
func (this Route) String() string   { return this.AllowedMethods.String() + " " + this.Path }
func (this Route) GoString() string { return this.String() }

const pipeDelimiter = "|"
