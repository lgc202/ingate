package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/admin/dto/gateway"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// ListGateway 返回 Gateway 列表
func (h *Handler) ListGateway(ctx *gin.Context) {
	gateways, err := h.services.Gateway.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list gateways failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询网关列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListGatewaysResp(gateways))
}

// GetGateway 返回单个 Gateway
func (h *Handler) GetGateway(ctx *gin.Context) {
	gatewayID := ctx.Param("id")
	gateway, err := h.services.Gateway.Get(ctx.Request.Context(), gatewayID)
	if err != nil {
		h.logger.Error("get gateway failed", "request_id", ctx.GetString(requestid.Header), "gateway_id", gatewayID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询网关失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetGatewayResp(gateway))
}

// CreateGateway 创建 Gateway
func (h *Handler) CreateGateway(ctx *gin.Context) {
	request := dto.CreateGatewayReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	id, err := h.services.Gateway.Create(ctx.Request.Context(), request.Spec())
	if err != nil {
		h.logger.Error("create gateway failed", "request_id", ctx.GetString(requestid.Header), "name", request.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建网关失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateGatewayResp{Success: true, ID: id})
}

// UpdateGateway 更新 Gateway
func (h *Handler) UpdateGateway(ctx *gin.Context) {
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
	err := h.services.Gateway.Update(ctx.Request.Context(), gatewayID, request.Version, request.Spec())
	if err != nil {
		h.logger.Error("update gateway failed", "request_id", ctx.GetString(requestid.Header), "gateway_id", gatewayID, "name", request.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新网关失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateGatewayResp{Success: true})
}

// SetGatewayEnabled 更新 Gateway 启停状态
func (h *Handler) SetGatewayEnabled(ctx *gin.Context) {
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
	err := h.services.Gateway.SetEnabled(ctx.Request.Context(), gatewayID, request.Value())
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

// DeleteGateway 删除 Gateway
func (h *Handler) DeleteGateway(ctx *gin.Context) {
	gatewayID := ctx.Param("id")
	err := h.services.Gateway.Delete(ctx.Request.Context(), gatewayID)
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
