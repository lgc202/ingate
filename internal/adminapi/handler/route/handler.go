package route

import (
	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
)

// Handler 处理 Route HTTP 请求
type Handler struct {
	service *routeservice.Service
}

// New 创建 Route handler
func New(service *routeservice.Service) *Handler {
	return &Handler{service: service}
}

// List 返回 Route 列表
func (h *Handler) List(ctx *gin.Context) {
	items, err := h.service.List(ctx.Request.Context())
	response.WriteResult(ctx, items, err)
}

// Get 返回单个 Route
func (h *Handler) Get(ctx *gin.Context) {
	item, err := h.service.Get(ctx.Request.Context(), ctx.Param("name"))
	response.WriteResult(ctx, item, err)
}
