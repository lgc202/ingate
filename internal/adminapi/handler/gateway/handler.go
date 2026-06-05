package gateway

import (
	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/gateway/dto"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
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
	request, err := bindCreateGatewayReq(ctx)
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err = h.service.Create(ctx.Request.Context(), createGatewayParams(request))
	response.WriteResult(ctx, dto.CreateGatewayResp{Success: true}, err)
}

// Update 更新 Gateway
func (h *Handler) Update(ctx *gin.Context) {
	request, err := bindUpdateGatewayReq(ctx)
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err = h.service.Update(ctx.Request.Context(), ctx.Param("name"), updateGatewayParams(request))
	response.WriteResult(ctx, dto.UpdateGatewayResp{Success: true}, err)
}

// SetEnabled 更新 Gateway 启停状态
func (h *Handler) SetEnabled(ctx *gin.Context) {
	request := dto.SetGatewayEnabledReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.WriteResult(ctx, nil, apierrors.NewBadRequest("invalid gateway enabled request body"))
		return
	}
	if err := request.Validate(); err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}

	err := h.service.SetEnabled(ctx.Request.Context(), ctx.Param("name"), request.Value())
	response.WriteResult(ctx, dto.SetGatewayEnabledResp{Success: true}, err)
}

// Delete 删除 Gateway
func (h *Handler) Delete(ctx *gin.Context) {
	err := h.service.Delete(ctx.Request.Context(), ctx.Param("name"))
	response.WriteResult(ctx, dto.DeleteGatewayResp{Success: true}, err)
}

// FormOptions 返回 Gateway 表单选项
func (h *Handler) FormOptions(ctx *gin.Context) {
	result, err := h.service.FormOptions(ctx.Request.Context())
	if err != nil {
		response.WriteResult(ctx, nil, err)
		return
	}
	response.WriteResult(ctx, dto.FromFormOptionsResult(result), nil)
}

func bindCreateGatewayReq(ctx *gin.Context) (dto.CreateGatewayReq, error) {
	request := dto.CreateGatewayReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		return dto.CreateGatewayReq{}, apierrors.NewBadRequest("invalid gateway request body")
	}
	if err := request.Validate(); err != nil {
		return dto.CreateGatewayReq{}, err
	}
	return request, nil
}

func bindUpdateGatewayReq(ctx *gin.Context) (dto.UpdateGatewayReq, error) {
	request := dto.UpdateGatewayReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		return dto.UpdateGatewayReq{}, apierrors.NewBadRequest("invalid gateway request body")
	}
	if err := request.Validate(); err != nil {
		return dto.UpdateGatewayReq{}, err
	}
	return request, nil
}

func createGatewayParams(request dto.CreateGatewayReq) gatewayservice.CreateGatewayParams {
	return gatewayservice.CreateGatewayParams{
		Name:         request.Name,
		Description:  request.Description,
		RuntimeGroup: request.RuntimeGroup,
		Listeners:    listenerParams(request.Listeners),
		HostBindings: hostBindingParams(request.HostBindings),
	}
}

func updateGatewayParams(request dto.UpdateGatewayReq) gatewayservice.UpdateGatewayParams {
	return gatewayservice.UpdateGatewayParams{
		Version:      request.Version,
		Description:  request.Description,
		RuntimeGroup: request.RuntimeGroup,
		Listeners:    listenerParams(request.Listeners),
		HostBindings: hostBindingParams(request.HostBindings),
	}
}

func listenerParams(listeners []dto.GatewayListenerReq) []gatewayservice.ListenerParams {
	params := make([]gatewayservice.ListenerParams, 0, len(listeners))
	for _, listener := range listeners {
		params = append(params, gatewayservice.ListenerParams{
			Name:     listener.Name,
			Protocol: resource.ListenerProtocol(listener.Protocol),
			Port:     listener.Port,
		})
	}
	return params
}

func hostBindingParams(bindings []dto.GatewayHostBindingReq) []gatewayservice.HostBindingParams {
	params := make([]gatewayservice.HostBindingParams, 0, len(bindings))
	for _, binding := range bindings {
		param := gatewayservice.HostBindingParams{
			Hostname:     binding.Hostname,
			ListenerRefs: append([]string(nil), binding.ListenerRefs...),
		}
		if binding.TLS != nil {
			param.CertificateRef = binding.TLS.CertificateRef
		}
		params = append(params, param)
	}
	return params
}
