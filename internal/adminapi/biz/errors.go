// Package biz 定义 ingate-admin-api 的业务规则和数据访问边界
package biz

import kratoserrors "github.com/go-kratos/kratos/v3/errors"

const (
	reasonInvalidArgument         = "INVALID_ARGUMENT"
	reasonRuleViolation           = "BUSINESS_RULE_VIOLATION"
	reasonResourceVersionConflict = "RESOURCE_VERSION_CONFLICT"
	reasonResourceNotFound        = "RESOURCE_NOT_FOUND"
)

var (
	// ErrResourceNotFound 表示声明式资源不存在
	ErrResourceNotFound = kratoserrors.NotFound(reasonResourceNotFound, "resource not found").
				WithMetadata(map[string]string{"user_message": "资源不存在或已被删除"})
	// ErrResourceVersionConflict 表示声明式资源已被其他请求修改
	ErrResourceVersionConflict = kratoserrors.Conflict(reasonResourceVersionConflict, "resource version conflict").
					WithMetadata(map[string]string{"user_message": "资源已被其他用户修改，请刷新后重试"})
	// ErrInvalidCursor 表示分页游标无法解析或已经失效
	ErrInvalidCursor = kratoserrors.BadRequest(reasonInvalidArgument, "invalid cursor").
				WithMetadata(map[string]string{"user_message": "分页游标无效或已过期"})
)

// NewRuleViolation 创建因当前系统状态或资源关系而拒绝请求的业务错误
func NewRuleViolation(userMessage string) error {
	return kratoserrors.Conflict(reasonRuleViolation, "request rejected").
		WithMetadata(map[string]string{"user_message": userMessage})
}

// NewVersionConflict 创建可以向用户说明的乐观锁冲突
func NewVersionConflict(resourceID, userMessage string) error {
	return kratoserrors.Conflict(reasonResourceVersionConflict, "resource version conflict").
		WithMetadata(map[string]string{
			"resource_id":  resourceID,
			"user_message": userMessage,
		})
}
