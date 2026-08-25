package conversation

import "errors"

var (
	ErrNotFound           = errors.New("conversation resource not found")
	ErrRunStateConflict   = errors.New("assistant run state conflict")
	ErrRunRunning         = errors.New("conversation already has an active run")
	ErrModelNotConfigured = errors.New("assistant model is not configured")
	ErrRunCancelled       = errors.New("assistant run cancellation requested")
	ErrRunLeaseLost       = errors.New("assistant run lease lost")

	errEventStoreUnavailable = errors.New("assistant event store is unavailable")
)
