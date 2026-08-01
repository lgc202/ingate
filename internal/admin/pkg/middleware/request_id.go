package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// RequestID 透传或生成请求 ID，并写回响应 header
func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id := ctx.GetHeader(requestid.Header)
		if id == "" {
			id = requestid.New()
		}
		ctx.Set(requestid.Header, id)
		ctx.Header(requestid.Header, id)
		ctx.Next()
	}
}
