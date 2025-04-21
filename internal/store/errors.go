package store

import "errors"

var (
	ErrNotFound  = errors.New("store: resource not found")
	ErrDuplicate = errors.New("store: duplicate resource")
	ErrConflict  = errors.New("store: conflicting resource state")
	// ErrOperationNotSupported was removed
	ErrForeignKeyViolation = errors.New("store: foreign key constraint violation")
)
