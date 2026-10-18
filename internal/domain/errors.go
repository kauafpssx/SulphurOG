package domain

import "errors"

var (
	ErrStorageFull       = errors.New("storage is full")
	ErrNotFound          = errors.New("resource not found")
	ErrPermanentFailure  = errors.New("permanent failure")
	ErrExtractionFailed  = errors.New("extraction failed")
	ErrUnsupportedFormat = errors.New("unsupported format")
)