package errno

import kratoserrors "github.com/go-kratos/kratos/v3/errors"

var (
	// ErrHTTPRouteInvalidArgument 对应路由参数不合法的公开结果。
	ErrHTTPRouteInvalidArgument = kratoserrors.BadRequest("HTTP_ROUTE_INVALID_ARGUMENT", "路由参数不合法")
	// ErrHTTPRouteAlreadyExists 对应资源标识冲突，请重新创建的公开结果。
	ErrHTTPRouteAlreadyExists = kratoserrors.Conflict("HTTP_ROUTE_ALREADY_EXISTS", "资源标识冲突，请重新创建")
	// ErrHTTPRouteNotFound 对应路由不存在的公开结果。
	ErrHTTPRouteNotFound = kratoserrors.NotFound("HTTP_ROUTE_NOT_FOUND", "路由不存在")
	// ErrHTTPRouteConflict 对应路由已被修改，请刷新后重试的公开结果。
	ErrHTTPRouteConflict = kratoserrors.Conflict("HTTP_ROUTE_CONFLICT", "路由已被修改，请刷新后重试")
)
