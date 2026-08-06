package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/admin/dto/accesscontrolpolicy"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// ListAccessControlPolicy 返回 AccessControlPolicy 列表
func (h *Handler) ListAccessControlPolicy(ctx *gin.Context) {
	result, err := h.services.AccessControlPolicy.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list access control policies failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询访问控制策略列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListAccessControlPoliciesResp(result))
}

// GetAccessControlPolicy 返回单个 AccessControlPolicy
func (h *Handler) GetAccessControlPolicy(ctx *gin.Context) {
	policyID := ctx.Param("id")
	result, err := h.services.AccessControlPolicy.Get(ctx.Request.Context(), policyID)
	if err != nil {
		h.logger.Error("get access control policy failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询访问控制策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetAccessControlPolicyResp(result))
}

// CreateAccessControlPolicy 创建 AccessControlPolicy
func (h *Handler) CreateAccessControlPolicy(ctx *gin.Context) {
	request := dto.CreateAccessControlPolicyReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	policyID, err := h.services.AccessControlPolicy.Create(ctx.Request.Context(), request.Spec())
	if err != nil {
		h.logger.Error("create access control policy failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建访问控制策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateAccessControlPolicyResp{Success: true, ID: policyID})
}

// UpdateAccessControlPolicy 更新 AccessControlPolicy
func (h *Handler) UpdateAccessControlPolicy(ctx *gin.Context) {
	request := dto.UpdateAccessControlPolicyReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	policyID := ctx.Param("id")
	err := h.services.AccessControlPolicy.Update(ctx.Request.Context(), policyID, request.Version, request.Spec())
	if err != nil {
		h.logger.Error("update access control policy failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新访问控制策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateAccessControlPolicyResp{Success: true})
}

// SetAccessControlPolicyEnabled 设置 AccessControlPolicy 启用状态
func (h *Handler) SetAccessControlPolicyEnabled(ctx *gin.Context) {
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
	err := h.services.AccessControlPolicy.SetEnabled(ctx.Request.Context(), policyID, enabled)
	if err != nil {
		h.logger.Error("set access control policy enabled failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "enabled", enabled, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新访问控制策略状态失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateAccessControlPolicyResp{Success: true})
}

// DeleteAccessControlPolicy 删除 AccessControlPolicy
func (h *Handler) DeleteAccessControlPolicy(ctx *gin.Context) {
	policyID := ctx.Param("id")
	err := h.services.AccessControlPolicy.Delete(ctx.Request.Context(), policyID)
	if err != nil {
		h.logger.Error("delete access control policy failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除访问控制策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteAccessControlPolicyResp{Success: true})
}
