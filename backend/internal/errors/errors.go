package errors

import (
	"fmt"
)

// // Check if err is instance of T for custom error types
// func Is[T error](err error) bool {
// 	if _, ok := err.(T); ok {
// 		return true
// 	}
// 	return false
// }

// default error is internal service error at handler level
// if error has different status code use ErrorWithStatusCode
type ErrorWithStatusCode struct {
	Message    string
	StatusCode int
}

func (e *ErrorWithStatusCode) Error() string {
	return fmt.Sprintf("Validation error: %s", e.Message)
}
