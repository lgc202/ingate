package run

import "errors"

var (
	ErrNotFound           = errors.New("assistant run not found")
	ErrStateConflict      = errors.New("assistant run state conflict")
	ErrConversationBusy   = errors.New("conversation already has an active run")
	ErrModelNotConfigured = errors.New("assistant model is not configured")
	ErrCancellation       = errors.New("assistant run cancellation requested")
	ErrLeaseLost          = errors.New("assistant run lease lost")
	ErrToolUnavailable    = errors.New("assistant tool is unavailable")

	errEventStoreUnavailable      = errors.New("assistant event store is unavailable")
	errExecutionRecordUnavailable = errors.New("assistant execution record is unavailable")
)
