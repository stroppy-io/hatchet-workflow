package domainerr

import (
	"errors"

	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
)

type Error struct {
	code    errorspb.Code
	details []*errorspb.Detail
	cause   error
}

func E(code errorspb.Code, details ...*errorspb.Detail) *Error {
	return &Error{code: code, details: details}
}

func (e *Error) Code() errorspb.Code         { return e.code }
func (e *Error) Details() []*errorspb.Detail { return e.details }
func (e *Error) Unwrap() error               { return e.cause }
func (e *Error) Error() string               { return e.code.String() }

func (e *Error) Is(target error) bool {
	var c *codefulErr
	if errors.As(target, &c) {
		return e.code == c.code
	}
	other, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.code == other.code
}

func (e *Error) WithCause(cause error) *Error {
	e.cause = cause
	return e
}
