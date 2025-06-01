package apperrors

import "errors"

var (
	ErrNotFound = errors.New("not found")

	ErrInvalidInput = errors.New("invalid input parameter")

	ErrInternalServer = errors.New("internal server error")

	// authentication error
	ErrUnauthorized = errors.New("unauthorized")

	// authorization error
	ErrForbidden = errors.New("forbidden")

	// Idempotency while creating orders
	ErrAlreadyExists = errors.New("already exists")
)
