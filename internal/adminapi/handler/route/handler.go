package route

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/route/dto"
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
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.FromListResult(result))
}

// PolicyCapabilities 返回当前后端支持的路由策略能力
func (h *Handler) PolicyCapabilities(ctx *gin.Context) {
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.PolicyCapabilities())
}

// Get 返回单个 Route
func (h *Handler) Get(ctx *gin.Context) {
	result, err := h.service.Get(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		h.logger.Error("get route failed", "request_id", ctx.GetString(requestid.Header), "route_id", ctx.Param("name"), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询路由失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.FromRouteResult(result))
}

// Create 创建 Route
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.RouteRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	route, err := request.Resource()
	if err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	err = h.service.Create(ctx.Request.Context(), route)
	if err != nil {
		h.logger.Error("create route failed", "request_id", ctx.GetString(requestid.Header), "route_id", route.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建路由失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.MutationResponse{Success: true})
}

// Update 更新 Route
func (h *Handler) Update(ctx *gin.Context) {
	request := dto.RouteRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	route, err := request.Resource()
	if err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	err = h.service.Update(ctx.Request.Context(), ctx.Param("name"), route)
	if err != nil {
		h.logger.Error("update route failed", "request_id", ctx.GetString(requestid.Header), "route_id", ctx.Param("name"), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新路由失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.MutationResponse{Success: true})
}

// SetEnabled 更新 Route 启停状态
func (h *Handler) SetEnabled(ctx *gin.Context) {
	request := dto.EnabledRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	err := h.service.SetEnabled(ctx.Request.Context(), ctx.Param("name"), request.Value())
	if err != nil {
		h.logger.Error("set route enabled failed", "request_id", ctx.GetString(requestid.Header), "route_id", ctx.Param("name"), "enabled", request.Value(), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新路由状态失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.MutationResponse{Success: true})
}

// Delete 删除 Route
func (h *Handler) Delete(ctx *gin.Context) {
	err := h.service.Delete(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		h.logger.Error("delete route failed", "request_id", ctx.GetString(requestid.Header), "route_id", ctx.Param("name"), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除路由失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.MutationResponse{Success: true})
}
