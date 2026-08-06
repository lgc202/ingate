package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/admin/dto/route"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// ListRoute 返回 Route 列表
func (h *Handler) ListRoute(ctx *gin.Context) {
	routes, err := h.services.Route.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list routes failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询路由列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListRoutesResp(routes))
}

// GetRoute 返回单个 Route
func (h *Handler) GetRoute(ctx *gin.Context) {
	routeID := ctx.Param("id")
	route, err := h.services.Route.Get(ctx.Request.Context(), routeID)
	if err != nil {
		h.logger.Error("get route failed", "request_id", ctx.GetString(requestid.Header), "route_id", routeID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询路由失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetRouteResp(route))
}

// CreateRoute 创建 Route
func (h *Handler) CreateRoute(ctx *gin.Context) {
	request := dto.CreateRouteReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	routeID, err := h.services.Route.Create(ctx.Request.Context(), request.Spec())
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

// UpdateRoute 更新 Route
func (h *Handler) UpdateRoute(ctx *gin.Context) {
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
	err := h.services.Route.Update(ctx.Request.Context(), routeID, request.Version, request.Spec())
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

// SetRouteEnabled 更新 Route 启停状态
func (h *Handler) SetRouteEnabled(ctx *gin.Context) {
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
	err := h.services.Route.SetEnabled(ctx.Request.Context(), routeID, request.Value())
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

// DeleteRoute 删除 Route
func (h *Handler) DeleteRoute(ctx *gin.Context) {
	routeID := ctx.Param("id")
	err := h.services.Route.Delete(ctx.Request.Context(), routeID)
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
