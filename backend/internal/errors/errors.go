package errors

import (
	"errors"
	"fmt"
)

var NotFound = errors.New("Not found")
var WrongPassword = errors.New("Bad password")

// Check if err is instance of T for custom error types
func Is[T error](err error) bool {
	if _, ok := err.(T); ok {
		return true
	}
	return false
}

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("Validation error: %s", e.Message)
}
