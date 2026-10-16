package errno

import kratoserrors "github.com/go-kratos/kratos/v3/errors"

var (
	// ErrGatewayInvalidArgument 对应网关参数不合法的公开结果。
	ErrGatewayInvalidArgument = kratoserrors.BadRequest("GATEWAY_INVALID_ARGUMENT", "网关参数不合法")
	// ErrGatewayAlreadyExists 对应资源标识冲突，请重新创建的公开结果。
	ErrGatewayAlreadyExists = kratoserrors.Conflict("GATEWAY_ALREADY_EXISTS", "资源标识冲突，请重新创建")
	// ErrGatewayNotFound 对应网关不存在的公开结果。
	ErrGatewayNotFound = kratoserrors.NotFound("GATEWAY_NOT_FOUND", "网关不存在")
	// ErrGatewayConflict 对应网关已被修改，请刷新后重试的公开结果。
	ErrGatewayConflict = kratoserrors.Conflict("GATEWAY_CONFLICT", "网关已被修改，请刷新后重试")
)
