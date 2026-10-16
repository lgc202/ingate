package errno

import kratoserrors "github.com/go-kratos/kratos/v3/errors"

var (
	// ErrMCPRouteInvalidArgument 对应MCP 路由参数不合法的公开结果。
	ErrMCPRouteInvalidArgument = kratoserrors.BadRequest("MCP_ROUTE_INVALID_ARGUMENT", "MCP 路由参数不合法")
	// ErrMCPRouteAlreadyExists 对应资源标识冲突，请重新创建的公开结果。
	ErrMCPRouteAlreadyExists = kratoserrors.Conflict("MCP_ROUTE_ALREADY_EXISTS", "资源标识冲突，请重新创建")
	// ErrMCPRouteNotFound 对应MCP 路由不存在的公开结果。
	ErrMCPRouteNotFound = kratoserrors.NotFound("MCP_ROUTE_NOT_FOUND", "MCP 路由不存在")
	// ErrMCPRouteConflict 对应MCP 路由已被修改，请刷新后重试的公开结果。
	ErrMCPRouteConflict = kratoserrors.Conflict("MCP_ROUTE_CONFLICT", "MCP 路由已被修改，请刷新后重试")
)
