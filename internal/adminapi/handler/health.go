package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
)

// Health 返回服务健康状态
func (h *Handler) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "requestID": ctx.GetString(requestid.Header)})
}
