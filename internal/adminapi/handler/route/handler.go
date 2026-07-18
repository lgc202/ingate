package route

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/adminapi/dto/route"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
)

// Handler 处理 Route HTTP 请求
type Handler struct {
	service *routeservice.Service
	logger  *slog.Logger
}

// New 创建 Route handler
func New(service *routeservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List 返回 Route 列表
func (h *Handler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list routes failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询路由列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListRoutesResp(result))
}

// Get 返回单个 Route
func (h *Handler) Get(ctx *gin.Context) {
	routeID := ctx.Param("id")
	result, err := h.service.Get(ctx.Request.Context(), routeID)
	if err != nil {
		h.logger.Error("get route failed", "request_id", ctx.GetString(requestid.Header), "route_id", routeID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询路由失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetRouteResp(result))
}

// Create 创建 Route
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreateRouteReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	routeID, err := h.service.Create(ctx.Request.Context(), request.Params())
	if err != nil {
		h.logger.Error("create route failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建路由失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateRouteResp{Success: true, ID: routeID})
}

// Update 更新 Route
func (h *Handler) Update(ctx *gin.Context) {
	request := dto.UpdateRouteReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	routeID := ctx.Param("id")
	err := h.service.Update(ctx.Request.Context(), routeID, request.Params())
	if err != nil {
		h.logger.Error("update route failed", "request_id", ctx.GetString(requestid.Header), "route_id", routeID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新路由失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateRouteResp{Success: true})
}

// SetEnabled 更新 Route 启停状态
func (h *Handler) SetEnabled(ctx *gin.Context) {
	request := dto.SetRouteEnabledReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	routeID := ctx.Param("id")
	err := h.service.SetEnabled(ctx.Request.Context(), routeID, request.Value())
	if err != nil {
		h.logger.Error("set route enabled failed", "request_id", ctx.GetString(requestid.Header), "route_id", routeID, "enabled", request.Value(), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新路由状态失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.SetRouteEnabledResp{Success: true})
}

// Delete 删除 Route
func (h *Handler) Delete(ctx *gin.Context) {
	routeID := ctx.Param("id")
	err := h.service.Delete(ctx.Request.Context(), routeID)
	if err != nil {
		h.logger.Error("delete route failed", "request_id", ctx.GetString(requestid.Header), "route_id", routeID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除路由失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteRouteResp{Success: true})
}
