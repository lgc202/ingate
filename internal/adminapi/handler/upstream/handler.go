package upstream

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/upstream/dto"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
)

// Handler 处理 Upstream HTTP 请求
type Handler struct {
	service *upstreamservice.Service
	logger  *slog.Logger
}

// New 创建 Upstream handler
func New(service *upstreamservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List 返回 Upstream 列表
func (h *Handler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list upstreams failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询上游列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.FromListResult(result))
}

// Get 返回单个 Upstream
func (h *Handler) Get(ctx *gin.Context) {
	result, err := h.service.Get(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		h.logger.Error("get upstream failed", "request_id", ctx.GetString(requestid.Header), "upstream_id", ctx.Param("name"), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询上游失败", nil)
		return
	}

	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.FromUpstreamResult(result))
}

// Create 创建 Upstream
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.UpstreamRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	upstream, err := request.Resource()
	if err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	err = h.service.Create(ctx.Request.Context(), upstream)
	if err != nil {
		h.logger.Error("create upstream failed", "request_id", ctx.GetString(requestid.Header), "upstream_id", upstream.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建上游失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.MutationResponse{Success: true})
}

// Update 更新 Upstream
func (h *Handler) Update(ctx *gin.Context) {
	request := dto.UpstreamRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	upstream, err := request.Resource()
	if err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	err = h.service.Update(ctx.Request.Context(), ctx.Param("name"), upstream)
	if err != nil {
		h.logger.Error("update upstream failed", "request_id", ctx.GetString(requestid.Header), "upstream_id", ctx.Param("name"), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新上游失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.MutationResponse{Success: true})
}

// Delete 删除 Upstream
func (h *Handler) Delete(ctx *gin.Context) {
	err := h.service.Delete(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		h.logger.Error("delete upstream failed", "request_id", ctx.GetString(requestid.Header), "upstream_id", ctx.Param("name"), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除上游失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.MutationResponse{Success: true})
}
