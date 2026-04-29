package gateway

import (
	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
)

// Handler 处理 Gateway HTTP 请求
type Handler struct {
	service *gatewayservice.Service
}

// New 创建 Gateway handler
func New(service *gatewayservice.Service) *Handler {
	return &Handler{service: service}
}

// List 返回 Gateway 列表
func (h *Handler) List(ctx *gin.Context) {
	items, err := h.service.List(ctx.Request.Context())
	response.WriteResult(ctx, items, err)
}

// Get 返回单个 Gateway
func (h *Handler) Get(ctx *gin.Context) {
	item, err := h.service.Get(ctx.Request.Context(), ctx.Param("name"))
	response.WriteResult(ctx, item, err)
}

// Overview 返回 Gateway 详情页聚合视图
func (h *Handler) Overview(ctx *gin.Context) {
	item, err := h.service.Overview(ctx.Request.Context(), ctx.Param("name"))
	response.WriteResult(ctx, item, err)
}
