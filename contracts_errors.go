package httprouter

import "errors"

var (
	ErrRouteExists       = errors.New("the method and path specified for this route already exists")
	ErrMalformedPath     = errors.New("the path specified for this route is malformed")
	ErrInvalidCharacters = errors.New("the path specified for this route contains invalid characters")
	ErrInvalidWildcard   = errors.New("the wildcard path specified must only contain a single asterisk '*'")
)
