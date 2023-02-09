#### SMARTY DISCLAIMER: Subject to the terms of the associated license agreement, this software is freely available for your use. This software is FREE, AS IN PUPPIES, and is a gift. Enjoy your new responsibility. This means that while we may consider enhancement requests, we may or may not choose to entertain requests at our sole and absolute discretion.

How to Use:
-----------------------

```
package main

import (
    "net/http"

    "github.com/smartystreets/httprouter"
)

func main() {
    var routes []httprouter.Route

    var createUserHandler http.Handler = nil // initialize this
    var userHandler http.Handler = nil       // initialize this
    routes = append(routes, httprouter.ParseRoutes("PUT", "/users|/old/path/to/users", createUserHandler)...)
    routes = append(routes, httprouter.ParseRoutes("GET|DELETE", "/users/:id", userHandler)...)

    var profileRouteHandler http.Handler = nil // initialize this
    routes = append(routes, httprouter.ParseRoutes("POST", "/users/*", profileRouteHandler)...)

    router, err := httprouter.New(
        httprouter.Options.Routes(routes...),
        httprouter.Options.MethodNotAllowed(&customMethodNotAllowedHandler{}), // optional
        httprouter.Options.NotFound(&customNotFoundHandler{}),                 // optional
        httprouter.Options.Recovery(panicRecovery),                            // optional
        httprouter.Options.Monitor(&routingMonitor{}))                         // optional

    if err != nil {
        panic(err)
    }

    _ = http.ListenAndServe("127.0.0.1:8080", router)
}

type customMethodNotAllowedHandler struct{}
type customNotFoundHandler struct{}
type routingMonitor struct{}

func (this *customMethodNotAllowedHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
func (this *customNotFoundHandler) ServeHTTP(http.ResponseWriter, *http.Request)         {}
func (*routingMonitor) Routed(*http.Request)                                             {}
func (*routingMonitor) NotFound(*http.Request)                                           {}
func (*routingMonitor) MethodNotAllowed(*http.Request)                                   {}
func (*routingMonitor) Recovered(*http.Request, interface{})                             {}
func panicRecovery(http.ResponseWriter, *http.Request, interface{})                      {}
```
