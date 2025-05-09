package errors

// default error is internal service error at handler level
// if error has different status code use ErrorWithStatusCode
type ErrorWithStatusCode struct {
	Message    string
	StatusCode int
}

func (e *ErrorWithStatusCode) Error() string {
	return e.Message
}

func IsNotFound(err error) bool {
	e, ok := err.(*ErrorWithStatusCode)
	return ok && e.StatusCode == 404
}
