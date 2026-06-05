package apperror

import "errors"

// BadRequest 表示用户输入或业务约束不满足
type BadRequest struct {
	message string
}

// NewBadRequest 创建用户输入错误
func NewBadRequest(message string) error {
	return BadRequest{message: message}
}

func (e BadRequest) Error() string {
	return e.message
}

// IsBadRequest 判断错误是否为用户输入错误
func IsBadRequest(err error) bool {
	var target BadRequest
	return errors.As(err, &target)
}
