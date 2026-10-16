package errno

import kratoserrors "github.com/go-kratos/kratos/v3/errors"

var (
	// ErrModelRouteInvalidArgument 对应模型路由参数不合法的公开结果。
	ErrModelRouteInvalidArgument = kratoserrors.BadRequest("MODEL_ROUTE_INVALID_ARGUMENT", "模型路由参数不合法")
	// ErrModelRouteAlreadyExists 对应资源标识冲突，请重新创建的公开结果。
	ErrModelRouteAlreadyExists = kratoserrors.Conflict("MODEL_ROUTE_ALREADY_EXISTS", "资源标识冲突，请重新创建")
	// ErrModelRouteNotFound 对应模型路由不存在的公开结果。
	ErrModelRouteNotFound = kratoserrors.NotFound("MODEL_ROUTE_NOT_FOUND", "模型路由不存在")
	// ErrModelRouteConflict 对应模型路由已被修改，请刷新后重试的公开结果。
	ErrModelRouteConflict = kratoserrors.Conflict("MODEL_ROUTE_CONFLICT", "模型路由已被修改，请刷新后重试")
)
