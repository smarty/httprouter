package httprouter

import "net/http"

type statusHandler int

func (this statusHandler) ServeHTTP(response http.ResponseWriter, _ *http.Request) {
	http.Error(response, http.StatusText(int(this)), int(this))
}

func RecoveryHandler(response http.ResponseWriter, _ *http.Request, _ interface{}) {
	http.Error(response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
