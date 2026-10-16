package server

import (
	"encoding/json"
	"errors"
	"net/http"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	"github.com/lgc202/ingate/internal/aiproxy/errno"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/apischema/openai"
)

// encodeModelError 只编码入口和 Handler 已映射的公开错误，不读取 cause 或 metadata。
// 上游响应不经过此编码器；流写出失败也不能在已开始的响应后追加 JSON。
func encodeModelError(w http.ResponseWriter, _ *http.Request, err error) {
	public, ok := errors.AsType[*kratoserrors.Error](err)
	if !ok {
		public = errno.ErrInternal
	}

	kind := "server_error"
	if public.Code < http.StatusInternalServerError {
		kind = "invalid_request_error"
	}

	response := openai.ErrorResponse{Error: openai.ErrorDetail{
		Message: public.Message,
		Type:    kind,
		Code:    new(public.Reason),
	}}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(int(public.Code))
	// ErrorEncoder 没有错误返回值；写出失败后不能再写第二份错误响应。
	// 此处仅含可编码的字符串字段，Encode 的失败来自响应写出，不重试或重复记录业务错误。
	_ = json.NewEncoder(w).Encode(response)
}
