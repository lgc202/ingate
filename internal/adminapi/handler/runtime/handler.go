package runtime

import (
	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	runtimeservice "github.com/lgc202/ingate/internal/adminapi/service/runtime"
)

// Handler 处理 RuntimeSnapshot HTTP 请求
type Handler struct {
	service *runtimeservice.Service
}

// New 创建 RuntimeSnapshot handler
func New(service *runtimeservice.Service) *Handler {
	return &Handler{service: service}
}

// List 返回 RuntimeSnapshot 列表
func (h *Handler) List(ctx *gin.Context) {
	items, err := h.service.List(ctx.Request.Context())
	response.WriteResult(ctx, items, err)
}

// Get 返回单个 RuntimeSnapshot
func (h *Handler) Get(ctx *gin.Context) {
	item, err := h.service.Get(ctx.Request.Context(), ctx.Param("name"))
	response.WriteResult(ctx, item, err)
}
