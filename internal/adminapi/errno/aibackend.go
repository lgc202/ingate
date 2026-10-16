package errno

import kratoserrors "github.com/go-kratos/kratos/v3/errors"

var (
	// ErrAIBackendInvalidArgument 对应模型服务参数不合法的公开结果。
	ErrAIBackendInvalidArgument = kratoserrors.BadRequest("AI_BACKEND_INVALID_ARGUMENT", "模型服务参数不合法")
	// ErrAIBackendAlreadyExists 对应资源标识冲突，请重新创建的公开结果。
	ErrAIBackendAlreadyExists = kratoserrors.Conflict("AI_BACKEND_ALREADY_EXISTS", "资源标识冲突，请重新创建")
	// ErrAIBackendNotFound 对应模型服务不存在的公开结果。
	ErrAIBackendNotFound = kratoserrors.NotFound("AI_BACKEND_NOT_FOUND", "模型服务不存在")
	// ErrAIBackendConflict 对应模型服务已被修改，请刷新后重试的公开结果。
	ErrAIBackendConflict = kratoserrors.Conflict("AI_BACKEND_CONFLICT", "模型服务已被修改，请刷新后重试")
)
