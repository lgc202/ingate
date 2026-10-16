package errno

import kratoserrors "github.com/go-kratos/kratos/v3/errors"

var (
	// ErrBackendInvalidArgument 对应后端资源参数不合法的公开结果。
	ErrBackendInvalidArgument = kratoserrors.BadRequest("BACKEND_INVALID_ARGUMENT", "后端资源参数不合法")
	// ErrBackendAlreadyExists 对应资源标识冲突，请重新创建的公开结果。
	ErrBackendAlreadyExists = kratoserrors.Conflict("BACKEND_ALREADY_EXISTS", "资源标识冲突，请重新创建")
	// ErrBackendNotFound 对应后端资源不存在的公开结果。
	ErrBackendNotFound = kratoserrors.NotFound("BACKEND_NOT_FOUND", "后端资源不存在")
	// ErrBackendConflict 对应后端资源已被修改，请刷新后重试的公开结果。
	ErrBackendConflict = kratoserrors.Conflict("BACKEND_CONFLICT", "后端资源已被修改，请刷新后重试")
)
