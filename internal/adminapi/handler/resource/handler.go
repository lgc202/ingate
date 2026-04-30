package resource

import (
	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	resourceservice "github.com/lgc202/ingate/internal/adminapi/service/resource"
)

// Handler 处理通用声明式资源 HTTP 请求
type Handler struct {
	service *resourceservice.Service
}

// New 创建通用声明式资源 handler
func New(service *resourceservice.Service) *Handler {
	return &Handler{service: service}
}

// List 返回资源列表 handler
func (h *Handler) List(kind resourceservice.Kind) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		items, err := h.service.List(ctx.Request.Context(), kind)
		response.WriteResult(ctx, items, err)
	}
}

// Get 返回单个资源 handler
func (h *Handler) Get(kind resourceservice.Kind) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		item, err := h.service.Get(ctx.Request.Context(), kind, ctx.Param("name"))
		response.WriteResult(ctx, item, err)
	}
}
