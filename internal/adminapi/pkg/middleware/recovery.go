package middleware

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
)

// Recovery 捕获 panic，避免单个请求击穿管理 API 进程
func Recovery(stdout io.Writer) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				id := ctx.GetString(requestid.Header)
				fmt.Fprintf(stdout, "admin-api panic requestID=%s error=%v\n", id, recovered)
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error", "requestID": id})
			}
		}()
		ctx.Next()
	}
}
