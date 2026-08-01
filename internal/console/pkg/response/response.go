package response

import (
	"github.com/gin-gonic/gin"
)

// ResponseWrapper 是 Console API 统一响应体
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
