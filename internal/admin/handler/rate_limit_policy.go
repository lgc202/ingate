package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/admin/dto/ratelimitpolicy"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// ListRateLimitPolicy 返回 RateLimitPolicy 列表
func (h *Handler) ListRateLimitPolicy(ctx *gin.Context) {
	result, err := h.services.RateLimitPolicy.List(ctx.Request.Context())
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

// GetRateLimitPolicy 返回单个 RateLimitPolicy
func (h *Handler) GetRateLimitPolicy(ctx *gin.Context) {
	policyID := ctx.Param("id")
	result, err := h.services.RateLimitPolicy.Get(ctx.Request.Context(), policyID)
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

// CreateRateLimitPolicy 创建 RateLimitPolicy
func (h *Handler) CreateRateLimitPolicy(ctx *gin.Context) {
	request := dto.CreateRateLimitPolicyReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	policyID, err := h.services.RateLimitPolicy.Create(ctx.Request.Context(), request.Spec())
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

// UpdateRateLimitPolicy 更新 RateLimitPolicy
func (h *Handler) UpdateRateLimitPolicy(ctx *gin.Context) {
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
	if err := h.services.RateLimitPolicy.Update(ctx.Request.Context(), policyID, request.Version, request.Spec()); err != nil {
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

// SetRateLimitPolicyEnabled 设置 RateLimitPolicy 启用状态
func (h *Handler) SetRateLimitPolicyEnabled(ctx *gin.Context) {
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
	enabled := request.Value()
	if err := h.services.RateLimitPolicy.SetEnabled(ctx.Request.Context(), policyID, enabled); err != nil {
		h.logger.Error("set rate limit policy enabled failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "enabled", enabled, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "设置限流策略状态失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.SetEnabledResp{Success: true})
}

// DeleteRateLimitPolicy 删除 RateLimitPolicy
func (h *Handler) DeleteRateLimitPolicy(ctx *gin.Context) {
	policyID := ctx.Param("id")
	if err := h.services.RateLimitPolicy.Delete(ctx.Request.Context(), policyID); err != nil {
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
