package httprouter

import (
	"net/http"
	"strings"
)

type Method uint16

func ParseMethods(value string) Method {
	var parsed Method

	for _, raw := range strings.Split(value, pipeDelimiter) {
		parsed |= ParseMethod(raw)
	}

	return parsed
}
func ParseMethod(value string) Method {
	value = strings.ToUpper(strings.TrimSpace(value))
	if parsed, found := availableMethods[value]; found {
		return parsed
	}

	return MethodNone
}
func (this Method) String() string {
	var result string

	for _, key := range orderedMethods {
		if key&this != key {
			continue
		}

		if len(result) > 0 {
			result += pipeDelimiter
		}

		result += methodValues[key]
	}

	return result
}
func (this Method) GoString() string { return this.String() }

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	MethodNone Method = 1 << iota
	MethodGet
	MethodHead
	MethodPost
	MethodPut
	MethodDelete
	MethodConnect
	MethodOptions
	MethodTrace
	MethodPatch
)

var (
	orderedMethods = []Method{
		MethodGet,
		MethodHead,
		MethodPost,
		MethodPut,
		MethodDelete,
		MethodConnect,
		MethodOptions,
		MethodTrace,
		MethodPatch,
	}

	methodValues = map[Method]string{
		MethodNone:    "",
		MethodGet:     http.MethodGet,
		MethodHead:    http.MethodHead,
		MethodPost:    http.MethodPost,
		MethodPut:     http.MethodPut,
		MethodDelete:  http.MethodDelete,
		MethodConnect: http.MethodConnect,
		MethodOptions: http.MethodOptions,
		MethodTrace:   http.MethodTrace,
		MethodPatch:   http.MethodPatch,
	}
	availableMethods = map[string]Method{
		"":                 MethodNone,
		http.MethodGet:     MethodGet,
		http.MethodHead:    MethodHead,
		http.MethodPost:    MethodPost,
		http.MethodPut:     MethodPut,
		http.MethodDelete:  MethodDelete,
		http.MethodConnect: MethodConnect,
		http.MethodOptions: MethodOptions,
		http.MethodTrace:   MethodTrace,
		http.MethodPatch:   MethodPatch,
	}
)
