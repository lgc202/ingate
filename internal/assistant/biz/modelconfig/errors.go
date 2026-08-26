package modelconfig

import "errors"

var (
	ErrNotConfigured     = errors.New("assistant model is not configured")
	ErrInvalidConnection = errors.New("assistant model connection is invalid")
)
