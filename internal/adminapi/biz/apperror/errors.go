// Package apperror 构造 Admin API 对调用方稳定且安全的业务错误。
package apperror

import adminv1 "github.com/lgc202/ingate/api/admin/v1"

// ResourceNotFound 返回声明式资源不存在错误。
func ResourceNotFound() error {
	return adminv1.ErrorResourceNotFound("资源不存在或已被删除")
}

// ResourceAlreadyExists 返回资源持久化身份已经存在错误。
func ResourceAlreadyExists() error {
	return adminv1.ErrorResourceAlreadyExists("资源已存在")
}

// ResourceVersionConflict 返回声明式资源已被其他请求修改错误。
func ResourceVersionConflict() error {
	return adminv1.ErrorResourceVersionConflict("资源已被其他用户修改，请刷新后重试")
}

// RequestRecordNotFound 返回请求记录不存在错误。
func RequestRecordNotFound() error {
	return adminv1.ErrorRequestRecordNotFound("请求记录不存在或已超过明细保留期")
}

// DependencyUnavailable 返回依赖不可用错误，并保留内部 cause。
func DependencyUnavailable(message string, cause error) error {
	err := adminv1.ErrorDependencyUnavailable("%s", message)
	if cause == nil {
		return err
	}
	return err.WithCause(cause)
}

// InvalidCursor 返回分页游标无法解析或已经失效错误。
func InvalidCursor(cause error) error {
	err := adminv1.ErrorInvalidArgument("分页游标无效或已过期")
	if cause == nil {
		return err
	}
	return err.WithCause(cause)
}

// InvalidResource 保留 API Server 的字段错误，同时只向控制台返回稳定提示。
func InvalidResource(cause error) error {
	return adminv1.ErrorInvalidArgument("配置内容不正确").WithCause(cause)
}
