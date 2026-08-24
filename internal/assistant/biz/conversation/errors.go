package conversation

import "errors"

var (
	ErrNotFound           = errors.New("conversation resource not found")
	ErrVersionConflict    = errors.New("conversation version conflict")
	ErrExecutionRunning   = errors.New("conversation already has a running execution")
	ErrModelNotConfigured = errors.New("assistant model is not configured")
)
