package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
)

// Recovery 捕获 panic，避免单个请求击穿管理 API 进程
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				id := ctx.GetString(requestid.Header)
				logger.Error("admin api panic recovered", "request_id", id, "err", recovered)
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error", "requestID": id})
			}
		}()
		ctx.Next()
	}
}
