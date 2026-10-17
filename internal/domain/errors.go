package domain

import "errors"

var (
	ErrStorageFull = errors.New("storage is full")
	ErrNotFound    = errors.New("resource not found")
)