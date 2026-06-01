package gateway

import (
	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/gateway/dto"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}
	response.WriteResult(ctx, dto.FromListResult(result), nil)
}

// Get 返回单个 Gateway
func (h *Handler) Get(ctx *gin.Context) {
	result, err := h.service.Get(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}
	response.WriteResult(ctx, dto.FromGatewayResult(result), nil)
}

// Create 创建 Gateway
func (h *Handler) Create(ctx *gin.Context) {
	request, ok := h.gatewayRequest(ctx)
	if !ok {
		return
	}
	gateway, err := request.Resource()
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err = h.service.Create(ctx.Request.Context(), gateway)
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

// Update 更新 Gateway
func (h *Handler) Update(ctx *gin.Context) {
	request, ok := h.gatewayRequest(ctx)
	if !ok {
		return
	}
	gateway, err := request.Resource()
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err = h.service.Update(ctx.Request.Context(), ctx.Param("name"), gateway)
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

// SetEnabled 更新 Gateway 启停状态
func (h *Handler) SetEnabled(ctx *gin.Context) {
	request := dto.EnabledRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.WriteResult(ctx, nil, apierrors.NewBadRequest("invalid gateway enabled request body"))
		return
	}
	if err := request.Validate(); err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err := h.service.SetEnabled(ctx.Request.Context(), ctx.Param("name"), request.Value())
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

// Delete 删除 Gateway
func (h *Handler) Delete(ctx *gin.Context) {
	err := h.service.Delete(ctx.Request.Context(), ctx.Param("name"))
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

// Overview 返回 Gateway 详情页聚合视图
func (h *Handler) Overview(ctx *gin.Context) {
	result, err := h.service.Overview(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}
	response.WriteResult(ctx, dto.FromDetailResult(result), nil)
}

func (h *Handler) gatewayRequest(ctx *gin.Context) (dto.GatewayRequest, bool) {
	request := dto.GatewayRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.WriteResult(ctx, nil, apierrors.NewBadRequest("invalid gateway request body"))
		return dto.GatewayRequest{}, false
	}
	if err := request.Validate(); err != nil {
		response.WriteResult(ctx, nil, err)
		return dto.GatewayRequest{}, false
	}
	return request, true
}
