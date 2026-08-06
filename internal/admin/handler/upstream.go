package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/admin/dto/upstream"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// ListUpstream 返回 Upstream 列表
func (h *Handler) ListUpstream(ctx *gin.Context) {
	upstreams, err := h.services.Upstream.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list upstreams failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询服务列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListUpstreamsResp(upstreams))
}

// GetUpstream 返回单个 Upstream
func (h *Handler) GetUpstream(ctx *gin.Context) {
	upstreamID := ctx.Param("id")
	upstream, err := h.services.Upstream.Get(ctx.Request.Context(), upstreamID)
	if err != nil {
		h.logger.Error("get upstream failed", "request_id", ctx.GetString(requestid.Header), "upstream_id", upstreamID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询服务失败", nil)
		return
	}

	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetUpstreamResp(upstream))
}

// CreateUpstream 创建 Upstream
func (h *Handler) CreateUpstream(ctx *gin.Context) {
	request := dto.CreateUpstreamReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	upstreamID, err := h.services.Upstream.Create(ctx.Request.Context(), request.Spec())
	if err != nil {
		h.logger.Error("create upstream failed", "request_id", ctx.GetString(requestid.Header), "name", request.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建服务失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateUpstreamResp{Success: true, ID: upstreamID})
}

// UpdateUpstream 更新 Upstream
func (h *Handler) UpdateUpstream(ctx *gin.Context) {
	request := dto.UpdateUpstreamReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	upstreamID := ctx.Param("id")
	err := h.services.Upstream.Update(
		ctx.Request.Context(),
		upstreamID,
		request.Version,
		request.Spec(),
		request.RemoveAPIKey,
	)
	if err != nil {
		h.logger.Error("update upstream failed", "request_id", ctx.GetString(requestid.Header), "upstream_id", upstreamID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新服务失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateUpstreamResp{Success: true})
}

// DeleteUpstream 删除 Upstream
func (h *Handler) DeleteUpstream(ctx *gin.Context) {
	upstreamID := ctx.Param("id")
	err := h.services.Upstream.Delete(ctx.Request.Context(), upstreamID)
	if err != nil {
		h.logger.Error("delete upstream failed", "request_id", ctx.GetString(requestid.Header), "upstream_id", upstreamID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除服务失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteUpstreamResp{Success: true})
}
