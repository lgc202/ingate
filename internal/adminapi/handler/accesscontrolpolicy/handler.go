package accesscontrolpolicy

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/adminapi/dto/accesscontrolpolicy"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	accesscontrolpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/accesscontrolpolicy"
)

// Handler 处理 AccessControlPolicy HTTP 请求
type Handler struct {
	service *accesscontrolpolicyservice.Service
	logger  *slog.Logger
}

// New 创建 AccessControlPolicy handler
func New(service *accesscontrolpolicyservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List 返回 AccessControlPolicy 列表
func (h *Handler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context())
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

// Get 返回单个 AccessControlPolicy
func (h *Handler) Get(ctx *gin.Context) {
	policyID := ctx.Param("id")
	result, err := h.service.Get(ctx.Request.Context(), policyID)
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

// Create 创建 AccessControlPolicy
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreateAccessControlPolicyReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	policyID, err := h.service.Create(ctx.Request.Context(), request.Spec())
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

// Update 更新 AccessControlPolicy
func (h *Handler) Update(ctx *gin.Context) {
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
	err := h.service.Update(ctx.Request.Context(), policyID, request.Version, request.Spec())
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

// SetEnabled 设置 AccessControlPolicy 启用状态
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
	enabled := request.Value()
	err := h.service.SetEnabled(ctx.Request.Context(), policyID, enabled)
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

// Delete 删除 AccessControlPolicy
func (h *Handler) Delete(ctx *gin.Context) {
	policyID := ctx.Param("id")
	err := h.service.Delete(ctx.Request.Context(), policyID)
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
