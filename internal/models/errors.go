package models

import (
	"errors"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrValidation      = errors.New("validation error")
	ErrUniqueViolation = errors.New("unique violation")

	ErrEmbeddingFailed = errors.New("embedding generation failed")
	ErrContentExists   = errors.New("content already exists")
)
