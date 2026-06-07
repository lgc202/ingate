package ratelimitpolicy

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/ratelimitpolicy/dto"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	ratelimitpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimitpolicy"
)

// Handler 处理 RateLimitPolicy HTTP 请求
type Handler struct {
	service *ratelimitpolicyservice.Service
	logger  *slog.Logger
}

// New 创建 RateLimitPolicy handler
func New(service *ratelimitpolicyservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List 返回 RateLimitPolicy 列表
func (h *Handler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list rate limit policies failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询限流策略列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListRateLimitPoliciesResp(result))
}

// Get 返回单个 RateLimitPolicy
func (h *Handler) Get(ctx *gin.Context) {
	policyID := ctx.Param("id")
	result, err := h.service.Get(ctx.Request.Context(), policyID)
	if err != nil {
		h.logger.Error("get rate limit policy failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询限流策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetRateLimitPolicyResp(result))
}

// Create 创建 RateLimitPolicy
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreateRateLimitPolicyReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	policyID, err := h.service.Create(ctx.Request.Context(), h.createParams(request))
	if err != nil {
		h.logger.Error("create rate limit policy failed", "request_id", ctx.GetString(requestid.Header), "name", request.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建限流策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateRateLimitPolicyResp{Success: true, ID: policyID})
}

// Update 更新 RateLimitPolicy
func (h *Handler) Update(ctx *gin.Context) {
	request := dto.UpdateRateLimitPolicyReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	policyID := ctx.Param("id")
	if err := h.service.Update(ctx.Request.Context(), policyID, h.updateParams(request)); err != nil {
		h.logger.Error("update rate limit policy failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新限流策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateRateLimitPolicyResp{Success: true})
}

// SetEnabled 设置 RateLimitPolicy 启用状态
func (h *Handler) SetEnabled(ctx *gin.Context) {
	request := dto.SetEnabledReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	policyID := ctx.Param("id")
	if err := h.service.SetEnabled(ctx.Request.Context(), policyID, request.Enabled); err != nil {
		h.logger.Error("set rate limit policy enabled failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "设置限流策略状态失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.SetEnabledResp{Success: true})
}

// Delete 删除 RateLimitPolicy
func (h *Handler) Delete(ctx *gin.Context) {
	policyID := ctx.Param("id")
	if err := h.service.Delete(ctx.Request.Context(), policyID); err != nil {
		h.logger.Error("delete rate limit policy failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除限流策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteRateLimitPolicyResp{Success: true})
}

func (h *Handler) createParams(request dto.CreateRateLimitPolicyReq) ratelimitpolicyservice.CreatePolicyParams {
	return ratelimitpolicyservice.CreatePolicyParams{PolicyParams: h.policyParams(request.RateLimitPolicyConfig)}
}

func (h *Handler) updateParams(request dto.UpdateRateLimitPolicyReq) ratelimitpolicyservice.UpdatePolicyParams {
	return ratelimitpolicyservice.UpdatePolicyParams{
		Version:      request.Version,
		PolicyParams: h.policyParams(request.RateLimitPolicyConfig),
	}
}

func (h *Handler) policyParams(config dto.RateLimitPolicyConfig) ratelimitpolicyservice.PolicyParams {
	return ratelimitpolicyservice.PolicyParams{
		Name:          config.Name,
		Description:   config.Description,
		Enabled:       config.Enabled,
		Mode:          config.Mode,
		Rules:         config.Rules,
		Global:        config.Global,
		Response:      config.Response,
		FailurePolicy: config.FailurePolicy,
	}
}
