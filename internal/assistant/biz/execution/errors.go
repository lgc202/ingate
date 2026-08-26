package execution

import "errors"

var (
	ErrNotFound           = errors.New("assistant execution not found")
	ErrStateConflict      = errors.New("assistant execution state conflict")
	ErrConversationBusy   = errors.New("conversation already has an active execution")
	ErrCancellation       = errors.New("assistant execution cancellation requested")
	ErrLeaseLost          = errors.New("assistant execution lease lost")
	ErrModelNotConfigured = errors.New("assistant model is not configured")
	ErrToolUnavailable    = errors.New("assistant tool is unavailable")

	errEventStoreUnavailable      = errors.New("assistant event store is unavailable")
	errExecutionRecordUnavailable = errors.New("assistant execution record is unavailable")
)
