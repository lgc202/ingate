package upstream

import (
	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/upstream/dto"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}
	response.WriteResult(ctx, dto.FromListResult(result), nil)
}

// Get 返回单个 Upstream
func (h *Handler) Get(ctx *gin.Context) {
	result, err := h.service.Get(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	response.WriteResult(ctx, dto.FromUpstreamResult(result), nil)
}

// Create 创建 Upstream
func (h *Handler) Create(ctx *gin.Context) {
	request, ok := h.upstreamRequest(ctx)
	if !ok {
		return
	}
	upstream, err := request.Resource()
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err = h.service.Create(ctx.Request.Context(), upstream)
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

// Update 更新 Upstream
func (h *Handler) Update(ctx *gin.Context) {
	request, ok := h.upstreamRequest(ctx)
	if !ok {
		return
	}
	upstream, err := request.Resource()
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err = h.service.Update(ctx.Request.Context(), ctx.Param("name"), upstream)
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

// Delete 删除 Upstream
func (h *Handler) Delete(ctx *gin.Context) {
	err := h.service.Delete(ctx.Request.Context(), ctx.Param("name"))
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

func (h *Handler) upstreamRequest(ctx *gin.Context) (dto.UpstreamRequest, bool) {
	request := dto.UpstreamRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.WriteResult(ctx, nil, apierrors.NewBadRequest("invalid service request body"))
		return dto.UpstreamRequest{}, false
	}
	if err := request.Validate(); err != nil {
		response.WriteResult(ctx, nil, err)
		return dto.UpstreamRequest{}, false
	}
	return request, true
}
