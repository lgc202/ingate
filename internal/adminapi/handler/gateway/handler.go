package gateway

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/gateway/dto"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Handler 处理 Gateway HTTP 请求
type Handler struct {
	service *gatewayservice.Service
	logger  *slog.Logger
}

// New 创建 Gateway handler
func New(service *gatewayservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List 返回 Gateway 列表
func (h *Handler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list gateways failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询网关列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListGatewaysResp(result))
}

// Get 返回单个 Gateway
func (h *Handler) Get(ctx *gin.Context) {
	gatewayID := ctx.Param("id")
	result, err := h.service.Get(ctx.Request.Context(), gatewayID)
	if err != nil {
		h.logger.Error("get gateway failed", "request_id", ctx.GetString(requestid.Header), "gateway_id", gatewayID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询网关失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetGatewayResp(result))
}

// Create 创建 Gateway
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreateGatewayReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	id, err := h.service.Create(ctx.Request.Context(), h.createGatewayParams(request))
	if err != nil {
		h.logger.Error("create gateway failed", "request_id", ctx.GetString(requestid.Header), "display_name", request.DisplayName, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建网关失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateGatewayResp{Success: true, ID: id})
}

// Update 更新 Gateway
func (h *Handler) Update(ctx *gin.Context) {
	request := dto.UpdateGatewayReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	gatewayID := ctx.Param("id")
	err := h.service.Update(ctx.Request.Context(), gatewayID, h.updateGatewayParams(request))
	if err != nil {
		h.logger.Error("update gateway failed", "request_id", ctx.GetString(requestid.Header), "gateway_id", gatewayID, "display_name", request.DisplayName, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新网关失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateGatewayResp{Success: true})
}

// SetEnabled 更新 Gateway 启停状态
func (h *Handler) SetEnabled(ctx *gin.Context) {
	request := dto.SetGatewayEnabledReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	gatewayID := ctx.Param("id")
	err := h.service.SetEnabled(ctx.Request.Context(), gatewayID, request.Value())
	if err != nil {
		h.logger.Error("set gateway enabled failed", "request_id", ctx.GetString(requestid.Header), "gateway_id", gatewayID, "enabled", request.Value(), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新网关状态失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.SetGatewayEnabledResp{Success: true})
}

// Delete 删除 Gateway
func (h *Handler) Delete(ctx *gin.Context) {
	gatewayID := ctx.Param("id")
	err := h.service.Delete(ctx.Request.Context(), gatewayID)
	if err != nil {
		h.logger.Error("delete gateway failed", "request_id", ctx.GetString(requestid.Header), "gateway_id", gatewayID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除网关失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteGatewayResp{Success: true})
}

func (h *Handler) createGatewayParams(request dto.CreateGatewayReq) gatewayservice.CreateGatewayParams {
	return gatewayservice.CreateGatewayParams{
		DisplayName:  request.DisplayName,
		Description:  request.Description,
		RuntimeGroup: request.RuntimeGroup,
		Listeners:    h.listenerParams(request.Listeners),
		HostBindings: h.hostBindingParams(request.HostBindings),
	}
}

func (h *Handler) updateGatewayParams(request dto.UpdateGatewayReq) gatewayservice.UpdateGatewayParams {
	return gatewayservice.UpdateGatewayParams{
		Version:      request.Version,
		DisplayName:  request.DisplayName,
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
			Protocol: resource.Protocol(listener.Protocol),
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
