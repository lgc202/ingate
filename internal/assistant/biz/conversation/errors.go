package conversation

import "errors"

var (
	ErrNotFound     = errors.New("conversation resource not found")
	ErrInvalidTitle = errors.New("conversation title is invalid")
)
