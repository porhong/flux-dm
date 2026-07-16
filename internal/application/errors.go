package application

import "fmt"

type ErrorCode string

const (
	ErrInvalidInput ErrorCode = "invalid_input"
	ErrUnavailable  ErrorCode = "unavailable"
	ErrInternal     ErrorCode = "internal"
)

// Error is safe to return across the Wails boundary. Cause remains backend-only.
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Cause   error     `json:"-"`
}

func NewError(code ErrorCode, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }
