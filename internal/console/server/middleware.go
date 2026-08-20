package server

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

func requestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id := ctx.GetHeader(requestid.Header)
		if id == "" {
			id = requestid.New()
		}
		ctx.Set(requestid.Header, id)
		ctx.Request.Header.Set(requestid.Header, id)
		ctx.Header(requestid.Header, id)
		ctx.Next()
	}
}

func cors() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,"+requestid.Header)
		ctx.Header("Access-Control-Expose-Headers", requestid.Header)

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		ctx.Next()
	}
}

func recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				id := ctx.GetString(requestid.Header)
				logger.Error("console request panic recovered", "request_id", id, "err", recovered)
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":     "internal server error",
					"requestID": id,
				})
			}
		}()
		ctx.Next()
	}
}
