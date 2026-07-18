package upstream

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"

	dto "github.com/lgc202/ingate/internal/adminapi/dto/upstream"
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
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询服务列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListUpstreamsResp(result))
}

// Get 返回单个 Upstream
func (h *Handler) Get(ctx *gin.Context) {
	upstreamID := ctx.Param("id")
	result, err := h.service.Get(ctx.Request.Context(), upstreamID)
	if err != nil {
		h.logger.Error("get upstream failed", "request_id", ctx.GetString(requestid.Header), "upstream_id", upstreamID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询服务失败", nil)
		return
	}

	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetUpstreamResp(result))
}

// Create 创建 Upstream
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreateUpstreamReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	upstreamID, err := h.service.Create(ctx.Request.Context(), h.createUpstreamParams(request))
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

// Update 更新 Upstream
func (h *Handler) Update(ctx *gin.Context) {
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
	err := h.service.Update(ctx.Request.Context(), upstreamID, h.updateUpstreamParams(request))
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

// Delete 删除 Upstream
func (h *Handler) Delete(ctx *gin.Context) {
	upstreamID := ctx.Param("id")
	err := h.service.Delete(ctx.Request.Context(), upstreamID)
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

func (h *Handler) createUpstreamParams(request dto.CreateUpstreamReq) upstreamservice.CreateUpstreamParams {
	return upstreamservice.CreateUpstreamParams{
		UpstreamParams: h.upstreamParams(request.UpstreamConfig),
	}
}

func (h *Handler) updateUpstreamParams(request dto.UpdateUpstreamReq) upstreamservice.UpdateUpstreamParams {
	return upstreamservice.UpdateUpstreamParams{
		Version:        request.Version,
		UpstreamParams: h.upstreamParams(request.UpstreamConfig),
	}
}

func (h *Handler) upstreamParams(config dto.UpstreamConfig) upstreamservice.UpstreamParams {
	return upstreamservice.UpstreamParams{
		Name:              config.Name,
		Type:              config.Type,
		LoadBalancePolicy: config.LoadBalancePolicy,
		Endpoints: lo.Map(config.Endpoints, func(endpoint dto.UpstreamEndpoint, _ int) upstreamservice.EndpointParams {
			return upstreamservice.EndpointParams{
				ID:      endpoint.ID,
				Address: endpoint.Address,
				Port:    endpoint.Port,
				Weight:  endpoint.Weight,
				Enabled: endpoint.Enabled,
			}
		}),
		HealthCheck: config.HealthCheck,
	}
}
