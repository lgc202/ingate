package gateway

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/gateway/dto"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
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
		h.writeServiceError(ctx, err)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListGatewaysResp(result))
}

// Get 返回单个 Gateway
func (h *Handler) Get(ctx *gin.Context) {
	result, err := h.service.Get(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		h.writeServiceError(ctx, err)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetGatewayResp(result))
}

// Create 创建 Gateway
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreateGatewayReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.WriteError(ctx, http.StatusBadRequest, "invalid gateway request body")
		return
	}
	if err := request.Validate(); err != nil {
		response.WriteError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	err := h.service.Create(ctx.Request.Context(), h.createGatewayParams(request))
	if err != nil {
		h.writeServiceError(ctx, err)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateGatewayResp{Success: true})
}

// Update 更新 Gateway
func (h *Handler) Update(ctx *gin.Context) {
	request := dto.UpdateGatewayReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.WriteError(ctx, http.StatusBadRequest, "invalid gateway request body")
		return
	}
	if err := request.Validate(); err != nil {
		response.WriteError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	err := h.service.Update(ctx.Request.Context(), ctx.Param("name"), h.updateGatewayParams(request))
	if err != nil {
		h.writeServiceError(ctx, err)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateGatewayResp{Success: true})
}

// SetEnabled 更新 Gateway 启停状态
func (h *Handler) SetEnabled(ctx *gin.Context) {
	request := dto.SetGatewayEnabledReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.WriteError(ctx, http.StatusBadRequest, "invalid gateway enabled request body")
		return
	}
	if err := request.Validate(); err != nil {
		response.WriteError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	err := h.service.SetEnabled(ctx.Request.Context(), ctx.Param("name"), request.Value())
	if err != nil {
		h.writeServiceError(ctx, err)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.SetGatewayEnabledResp{Success: true})
}

// Delete 删除 Gateway
func (h *Handler) Delete(ctx *gin.Context) {
	err := h.service.Delete(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		h.writeServiceError(ctx, err)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteGatewayResp{Success: true})
}

// FormOptions 返回 Gateway 表单选项
func (h *Handler) FormOptions(ctx *gin.Context) {
	result, err := h.service.FormOptions(ctx.Request.Context())
	if err != nil {
		h.writeServiceError(ctx, err)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetGatewayFormOptionsResp(result))
}

func (h *Handler) writeServiceError(ctx *gin.Context, err error) {
	if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, userError.Error(), nil)
		return
	}
	if apierrors.IsNotFound(err) {
		response.GinAbortJSONResponse(ctx, http.StatusNotFound, "resource not found", nil)
		return
	}
	if apierrors.IsAlreadyExists(err) || apierrors.IsConflict(err) {
		response.GinAbortJSONResponse(ctx, http.StatusConflict, err.Error(), nil)
		return
	}
	response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "gateway operation failed", nil)
}

func (h *Handler) createGatewayParams(request dto.CreateGatewayReq) gatewayservice.CreateGatewayParams {
	return gatewayservice.CreateGatewayParams{
		Name:         request.Name,
		Description:  request.Description,
		RuntimeGroup: request.RuntimeGroup,
		Listeners:    h.listenerParams(request.Listeners),
		HostBindings: h.hostBindingParams(request.HostBindings),
	}
}

func (h *Handler) updateGatewayParams(request dto.UpdateGatewayReq) gatewayservice.UpdateGatewayParams {
	return gatewayservice.UpdateGatewayParams{
		Version:      request.Version,
		Description:  request.Description,
		RuntimeGroup: request.RuntimeGroup,
		Listeners:    h.listenerParams(request.Listeners),
		HostBindings: h.hostBindingParams(request.HostBindings),
	}
}

func (h *Handler) listenerParams(listeners []dto.GatewayListenerReq) []gatewayservice.ListenerParams {
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

func (h *Handler) hostBindingParams(bindings []dto.GatewayHostBindingReq) []gatewayservice.HostBindingParams {
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
