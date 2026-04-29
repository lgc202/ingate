package upstream

import (
	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
)

// Handler 处理 Upstream HTTP 请求
type Handler struct {
	service *upstreamservice.Service
}

// New 创建 Upstream handler
func New(service *upstreamservice.Service) *Handler {
	return &Handler{service: service}
}

// List 返回 Upstream 列表
func (h *Handler) List(ctx *gin.Context) {
	items, err := h.service.List(ctx.Request.Context())
	response.WriteResult(ctx, items, err)
}

// Get 返回单个 Upstream
func (h *Handler) Get(ctx *gin.Context) {
	item, err := h.service.Get(ctx.Request.Context(), ctx.Param("name"))
	response.WriteResult(ctx, item, err)
}
