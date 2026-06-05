package route

import (
	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/route/dto"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}
	response.WriteResult(ctx, dto.FromListResult(result), nil)
}

// PolicyCapabilities 返回当前后端支持的路由策略能力
func (h *Handler) PolicyCapabilities(ctx *gin.Context) {
	response.WriteResult(ctx, dto.PolicyCapabilities(), nil)
}

// Get 返回单个 Route
func (h *Handler) Get(ctx *gin.Context) {
	result, err := h.service.Get(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}
	response.WriteResult(ctx, dto.FromRouteResult(result), nil)
}

// Create 创建 Route
func (h *Handler) Create(ctx *gin.Context) {
	request, err := h.routeRequest(ctx)
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}
	route, err := request.Resource()
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err = h.service.Create(ctx.Request.Context(), route)
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

// Update 更新 Route
func (h *Handler) Update(ctx *gin.Context) {
	request, err := h.routeRequest(ctx)
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}
	route, err := request.Resource()
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err = h.service.Update(ctx.Request.Context(), ctx.Param("name"), route)
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

// SetEnabled 更新 Route 启停状态
func (h *Handler) SetEnabled(ctx *gin.Context) {
	request := dto.EnabledRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.WriteResult(ctx, nil, apierrors.NewBadRequest("invalid route enabled request body"))
		return
	}
	if err := request.Validate(); err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err := h.service.SetEnabled(ctx.Request.Context(), ctx.Param("name"), request.Value())
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

// Delete 删除 Route
func (h *Handler) Delete(ctx *gin.Context) {
	err := h.service.Delete(ctx.Request.Context(), ctx.Param("name"))
	response.WriteResult(ctx, dto.MutationResponse{Success: true}, err)
}

func (h *Handler) routeRequest(ctx *gin.Context) (dto.RouteRequest, error) {
	request := dto.RouteRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		return dto.RouteRequest{}, apierrors.NewBadRequest("invalid route request body")
	}
	if err := request.Validate(); err != nil {
		return dto.RouteRequest{}, err
	}
	return request, nil
}
