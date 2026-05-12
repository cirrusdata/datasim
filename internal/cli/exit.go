package cli

import "errors"

type exitCoder interface {
	error
	ExitCode() int
}

type exitError struct {
	code    int
	message string
}

// Error returns the exit error message.
func (e *exitError) Error() string {
	return e.message
}

// ExitCode returns the process exit code for the error.
func (e *exitError) ExitCode() int {
	return e.code
}

// newExitError constructs an error that carries a specific process exit code.
func newExitError(code int, message string) error {
	return &exitError{code: code, message: message}
}

// ExitCode returns the desired process exit code for the provided error.
func ExitCode(err error) int {
	var coder exitCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}

	return 1
}
