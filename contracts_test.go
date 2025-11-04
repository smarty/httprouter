package httprouter

import (
	"reflect"
	"testing"
)

func TestParseMethods(t *testing.T) {
	assertParsedMethods(t, "GET|PUT|OPTIONS", MethodGet|MethodPut|MethodOptions)
	assertParsedMethods(t, "head | post  | trace", MethodHead|MethodPost|MethodTrace)
	assertParsedMethods(t, "\tconnect\n | \n\tunknown \n|\n delete", MethodConnect|MethodNone|MethodDelete)

	method := MethodPatch | MethodHead | MethodOptions
	Assert(t).That(method.String()).Equals("HEAD|OPTIONS|PATCH")
	Assert(t).That(method.String()).Equals(method.GoString())
	Assert(t).That(MethodNone.String()).Equals("")
}
func assertParsedMethods(t *testing.T, raw string, expected Method) {
	parsed := ParseMethods(raw)
	Assert(t).That(parsed).Equals(expected)
}

func TestParseRoutes(t *testing.T) {
	assertParsedRoutes(t, "GET|PUT", "/resource|/another/resource",
		Route{AllowedMethods: MethodGet | MethodPut, Path: "/resource"},
		Route{AllowedMethods: MethodGet | MethodPut, Path: "/another/resource"})

	assertParsedRoutes(t, "HEAD|OPTIONS", " \n\t  /Path/To/Document | \n\t /Document/* \t\n",
		Route{AllowedMethods: MethodHead | MethodOptions, Path: "/Path/To/Document"},
		Route{AllowedMethods: MethodHead | MethodOptions, Path: "/Document/*"})

	route := ParseRoute("GET|HEAD", "/document", nil)
	Assert(t).That(route.String()).Equals("GET|HEAD /document")
	Assert(t).That(route.String()).Equals(route.GoString())

}
func assertParsedRoutes(t *testing.T, methods, paths string, expectedRoutes ...Route) {
	parsed := ParseRoutes(methods, paths, nil)

	Assert(t).That(len(parsed)).Equals(len(expectedRoutes))

	for i := range parsed {
		Assert(t).That(parsed[i]).Equals(expectedRoutes[i])
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type That struct{ t *testing.T }
type Assertion struct {
	*testing.T
	actual any
}

func Assert(t *testing.T) *That               { return &That{t: t} }
func (this *That) That(actual any) *Assertion { return &Assertion{T: this.t, actual: actual} }

func (this *Assertion) IsNil() {
	this.Helper()
	if this.actual != nil && !reflect.ValueOf(this.actual).IsNil() {
		this.Equals(nil)
	}
}
func (this *Assertion) Equals(expected any) {
	this.Helper()
	if !reflect.DeepEqual(this.actual, expected) {
		this.Errorf("\nExpected: %#v\nActual:   %#v", expected, this.actual)
	}
}
