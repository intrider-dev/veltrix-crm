package errx

import "errors"

var (
	ErrUnauthenticated     = errors.New("unauthenticated")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrForbidden           = errors.New("forbidden")
	ErrNotFound            = errors.New("not found")
	ErrVersionConflict     = errors.New("version conflict")
	ErrIdempotencyConflict = errors.New("idempotency conflict")
	ErrRateLimited         = errors.New("rate limited")
	ErrSecurityRejected    = errors.New("security policy rejected request")
	ErrConflict            = errors.New("conflict")
	ErrUnavailable         = errors.New("temporarily unavailable")
)

type ValidationError struct {
	Fields []FieldError
}

type FieldError struct {
	Pointer string
	Code    string
	Params  map[string]any
}

func (err *ValidationError) Error() string { return "validation failed" }
