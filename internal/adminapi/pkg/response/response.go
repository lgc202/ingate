package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
)

// ResponseWrapper 是 admin-api 统一响应体
type ResponseWrapper struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// GinJSONResponse 返回统一 JSON 响应
func GinJSONResponse(ctx *gin.Context, code int, msg string, data any) {
	ctx.JSON(code, NewResponseWrapper(code, msg, data))
}

// GinAbortJSONResponse 终止请求并返回统一 JSON 响应
func GinAbortJSONResponse(ctx *gin.Context, code int, msg string, data any) {
	if _, ok := data.(error); ok {
		data = nil
	}
	ctx.AbortWithStatusJSON(code, NewResponseWrapper(code, msg, data))
}

// NewResponseWrapper 创建统一响应体
func NewResponseWrapper(code int, msg string, data any) *ResponseWrapper {
	return &ResponseWrapper{
		Code: code,
		Msg:  msg,
		Data: data,
	}
}

// WriteResult 兼容旧 handler 的统一响应入口
func WriteResult(ctx *gin.Context, value any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		if apierrors.IsNotFound(err) {
			status = http.StatusNotFound
		}
		if apierrors.IsBadRequest(err) {
			status = http.StatusBadRequest
		}
		if apierrors.IsAlreadyExists(err) || apierrors.IsConflict(err) {
			status = http.StatusConflict
		}
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			GinAbortJSONResponse(ctx, status, userError.Error(), nil)
			return
		}
		GinAbortJSONResponse(ctx, status, err.Error(), nil)
		return
	}
	GinJSONResponse(ctx, http.StatusOK, "ok", value)
}

// WriteError 兼容旧 handler 的错误响应入口
func WriteError(ctx *gin.Context, status int, message string) {
	GinAbortJSONResponse(ctx, status, message, nil)
}
