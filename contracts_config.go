package httprouter

import "net/http"

func RequireNew(options ...Option) http.Handler {
	if handler, err := New(options...); err != nil {
		panic(err)
	} else {
		return handler
	}
}
func New(options ...Option) (http.Handler, error) {
	var config configuration
	Options.apply(options...)(&config)

	treeRoot := &treeNode{}
	for _, route := range config.Routes {
		// FUTURE: add a "prune" function to where nodes with a single child are combined
		// this would be called after all calls to Add have been completed which would finalize the tree.
		if err := treeRoot.Add(route); err != nil {
			return nil, err
		}
	}

	router := newRouter(treeRoot, config.NotFound, config.MethodNotAllowed, config.Monitor)
	if config.Recovery == nil {
		return router, nil
	}

	return newRecoveryRouter(router, config.Recovery, config.Monitor), nil
}

func (singleton) AddRoute(method, path string, handler http.Handler) Option {
	return func(this *configuration) { this.Routes = append(this.Routes, ParseRoutes(method, path, handler)...) }
}
func (singleton) Routes(value ...Route) Option {
	return func(this *configuration) { this.Routes = append(this.Routes, value...) } // can be empty
}
func (singleton) MethodNotAllowed(value http.Handler) Option {
	return func(this *configuration) { this.MethodNotAllowed = value } // must not be nil
}
func (singleton) NotFound(value http.Handler) Option {
	return func(this *configuration) { this.NotFound = value } // must not be nil
}
func (singleton) Recovery(value RecoveryFunc) Option {
	return func(this *configuration) { this.Recovery = value } // can be nil which means to not handle a panic
}
func (singleton) Monitor(value Monitor) Option {
	return func(this *configuration) { this.Monitor = value }
}

func (singleton) apply(options ...Option) Option {
	return func(this *configuration) {
		for _, item := range Options.defaults(options...) {
			item(this)
		}
	}
}
func (singleton) defaults(options ...Option) []Option {
	return append([]Option{
		Options.NotFound(statusHandler(http.StatusNotFound)),
		Options.MethodNotAllowed(statusHandler(http.StatusMethodNotAllowed)),
		Options.Recovery(nil), // by default, don't handle a panic
		Options.Monitor(&nop{}),
	}, options...)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type configuration struct {
	Routes           []Route
	NotFound         http.Handler
	MethodNotAllowed http.Handler
	Recovery         RecoveryFunc
	Monitor          Monitor
}
type Option func(*configuration)
type singleton struct{}

var Options singleton

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type nop struct{}

func (*nop) Routed(*http.Request)           {}
func (*nop) NotFound(*http.Request)         {}
func (*nop) MethodNotAllowed(*http.Request) {}
func (*nop) Recovered(*http.Request, any)   {}
