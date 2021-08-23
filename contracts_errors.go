package httprouter

import "errors"

var (
	ErrRouteMethod             = errors.New("the method specified for this route is not valid")
	ErrRouteMissingPath        = errors.New("the path specified for this route is missing")
	ErrPathMissingLeadingSlash = errors.New("the path specified for this route requires a leading '/' (slash) character")
)
