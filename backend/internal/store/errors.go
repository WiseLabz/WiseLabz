package store

import "errors"

// Sentinel errors returned by repository methods.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)
