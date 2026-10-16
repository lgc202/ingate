// Package errno 定义 AI Proxy 本地产生的公开协议错误，不转换上游响应或流式中断。
// 包级定义只读使用；需要附加内部原因时调用 Kratos 的 WithCause，不直接修改字段。
package errno

import (
	"net/http"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

var (
	// ErrInvalidModelEntry 表示可信入口 Header 缺失或无效。
	ErrInvalidModelEntry = kratoserrors.BadRequest("INVALID_MODEL_ENTRY", "invalid model entry")
	// ErrModelEntryUnavailable 表示请求 Context 中没有已验证的模型入口。
	ErrModelEntryUnavailable = kratoserrors.ServiceUnavailable("MODEL_ENTRY_UNAVAILABLE", "model entry is unavailable")
	// ErrModelEncodingUnsupported 表示请求正文使用了不支持的压缩编码。
	ErrModelEncodingUnsupported = kratoserrors.New(
		http.StatusUnsupportedMediaType,
		"MODEL_ENCODING_UNSUPPORTED",
		"compressed model requests are not supported",
	)
	// ErrModelRequestTooLarge 表示模型请求正文超过配置的字节上限。
	ErrModelRequestTooLarge = kratoserrors.New(
		http.StatusRequestEntityTooLarge,
		"MODEL_REQUEST_TOO_LARGE",
		"model request body is too large",
	)
	// ErrModelRequestReadFailed 表示模型请求正文读取失败。
	ErrModelRequestReadFailed = kratoserrors.BadRequest("MODEL_REQUEST_READ_FAILED", "cannot read model request body")
	// ErrModelInvalidArgument 表示请求不符合模型入口契约。
	ErrModelInvalidArgument = kratoserrors.BadRequest("MODEL_INVALID_ARGUMENT", "invalid model request")
	// ErrModelNotFound 表示当前入口没有匹配请求模型的路由。
	ErrModelNotFound = kratoserrors.NotFound("MODEL_NOT_FOUND", "model is not available at this entry")
	// ErrModelUnavailable 表示模型配置或选中的后端不可用。
	ErrModelUnavailable = kratoserrors.ServiceUnavailable("MODEL_UNAVAILABLE", "model service is unavailable")
	// ErrRequestCanceled 表示请求已被取消。
	ErrRequestCanceled = kratoserrors.ClientClosed("REQUEST_CANCELED", "request was canceled")
	// ErrDeadlineExceeded 表示请求处理超过截止时间。
	ErrDeadlineExceeded = kratoserrors.GatewayTimeout("DEADLINE_EXCEEDED", "request deadline exceeded")
	// ErrModelForwardFailed 表示调用 Envoy 模型出口失败。
	ErrModelForwardFailed = kratoserrors.New(http.StatusBadGateway, "MODEL_FORWARD_FAILED", "model forwarding failed")
	// ErrInternal 隐藏未知内部错误的实现细节。
	ErrInternal = kratoserrors.InternalServer("INTERNAL_SERVER_ERROR", "internal server error")
)
