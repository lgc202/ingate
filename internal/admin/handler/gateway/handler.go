package gateway

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/admin/dto/gateway"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	gatewayservice "github.com/lgc202/ingate/internal/admin/service/gateway"
	"github.com/lgc202/ingate/internal/pkg/requestid"
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
	gateways, err := h.service.List(ctx.Request.Context())
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

// Get 返回单个 Gateway
func (h *Handler) Get(ctx *gin.Context) {
	gatewayID := ctx.Param("id")
	gateway, err := h.service.Get(ctx.Request.Context(), gatewayID)
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

	id, err := h.service.Create(ctx.Request.Context(), request.Spec())
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
	err := h.service.Update(ctx.Request.Context(), gatewayID, request.Version, request.Spec())
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
