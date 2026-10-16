package errno

import kratoserrors "github.com/go-kratos/kratos/v3/errors"

var (
	// ErrAuthInvalidCredentials 不区分账户不存在和密码错误。
	ErrAuthInvalidCredentials = kratoserrors.Unauthorized("AUTH_INVALID_CREDENTIALS", "用户名或密码错误")
	// ErrAuthSessionRequired 表示凭证缺失、过期或已经撤销。
	ErrAuthSessionRequired = kratoserrors.Unauthorized("AUTH_SESSION_REQUIRED", "请先登录")
	// ErrAuthUnavailable 表示认证依赖不可用，不把故障伪装成凭证错误。
	ErrAuthUnavailable = kratoserrors.ServiceUnavailable("AUTH_UNAVAILABLE", "认证服务暂时不可用")
	// ErrCurrentPasswordInvalid 表示改密时提供的当前密码错误。
	ErrCurrentPasswordInvalid = kratoserrors.BadRequest("CURRENT_PASSWORD_INVALID", "当前密码错误")
	// ErrPasswordUnchanged 表示新旧密码相同。
	ErrPasswordUnchanged = kratoserrors.BadRequest("PASSWORD_UNCHANGED", "新密码不能与当前密码相同")
)
