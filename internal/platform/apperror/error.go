package apperror

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrRateLimited  = errors.New("rate limited")
	ErrUnavailable  = errors.New("unavailable")
)

// Error is a safe client message wrapped around a sentinel.
type Error struct {
	sentinel error
	message  string
	code     string
}

func Invalid(message string) error {
	return &Error{sentinel: ErrInvalidInput, message: message}
}

func Unauthorized(message string) error {
	return &Error{sentinel: ErrUnauthorized, message: message}
}

func Forbidden(message string) error {
	return &Error{sentinel: ErrForbidden, message: message}
}

func Conflict(message string) error {
	return &Error{sentinel: ErrConflict, message: message}
}

func ConflictCode(code, message string) error {
	return &Error{sentinel: ErrConflict, message: message, code: code}
}

func Unavailable(message string) error {
	return &Error{sentinel: ErrUnavailable, message: message}
}

func (e *Error) Error() string { return e.message }

func (e *Error) Unwrap() error { return e.sentinel }

func (e *Error) Message() string { return e.message }

func (e *Error) Code() string { return e.code }
