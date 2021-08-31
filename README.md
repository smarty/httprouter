How to Use:
-----------------------

```
package main

import (
   "net/http"
   
   "github.com/smartystreets/httprouter"
)

func main() {
    var routes []httprouter.Routes

    var createUserHandler http.Handler = nil // initialize this
    var userHandler http.Handler = nil       // initialize this
    routes = append(routes, httprouter.ParseRoutes("PUT", "/users", createUserHandler))
    routes = append(routes, httprouter.ParseRoutes("GET|DELETE", "/users/:id", userHandler))
    
    var profileRouteHandler http.Handler = nil // initialize this
    routes = append(routes, httprouter.ParseRoutes("POST", "/users/*", profileRouteHandler))

    router := httprouter.New(
        httprouter.Options.Route(routes...),
        httprouter.Options.MethodNotAllowed(&customMethodNotAllowedHandler{}), // optional
        httprouter.Options.NotFound(&customNotFoundHandler{}),                 // optional
        httprouter.Options.Recovery(panicRecovery),                            // optional
        httprouter.Options.Monitor(&routingMonitor{}))                         // optional

    _ = http.ListenAndServe("127.0.0.1:8080", router)
}

type customMethodNotAllowedHandler struct {}
func (this *notFoundHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {}


type customNotFoundHandler struct {}
func (this *customNotFoundHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {}


func panicRecovery(response http.ResponseWriter, request *http.Request, recovered interface{}) {
}

type routingMonitor struct {

func (*routingMonitor) Routed(*http.Request)                 {}
func (*routingMonitor) NotFound(*http.Request)               {}
func (*routingMonitor) MethodNotAllowed(*http.Request)       {}
func (*routingMonitor) Recovered(*http.Request, interface{}) {}


```