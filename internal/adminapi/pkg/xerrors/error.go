package xerrors

// UserError 表示错误消息可以直接展示给用户
type UserError struct {
	message string
}

// NewUserError 创建可展示给用户的错误
func NewUserError(message string) *UserError {
	return &UserError{message: message}
}

func (e *UserError) Error() string {
	return e.message
}
