package tokenquotapolicy

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/admin/dto/tokenquotapolicy"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	tokenquotapolicyservice "github.com/lgc202/ingate/internal/admin/service/tokenquotapolicy"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// Handler 处理 TokenQuotaPolicy HTTP 请求
type Handler struct {
	service *tokenquotapolicyservice.Service
	logger  *slog.Logger
}

// New 创建 TokenQuotaPolicy handler
func New(service *tokenquotapolicyservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List 返回 TokenQuotaPolicy 列表
func (h *Handler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list token quota policies failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询 Token 配额策略列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListTokenQuotaPoliciesResp(result))
}

// Get 返回单个 TokenQuotaPolicy
func (h *Handler) Get(ctx *gin.Context) {
	path := dto.IDReq{}
	if err := ctx.ShouldBindUri(&path); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := path.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	policyID := path.ID
	result, err := h.service.Get(ctx.Request.Context(), policyID)
	if err != nil {
		h.logger.Error("get token quota policy failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询 Token 配额策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetTokenQuotaPolicyResp(result))
}

// Create 创建 TokenQuotaPolicy
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreateTokenQuotaPolicyReq{}
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
		h.logger.Error("create token quota policy failed", "request_id", ctx.GetString(requestid.Header), "name", request.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建 Token 配额策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateTokenQuotaPolicyResp{Success: true, ID: policyID})
}

// Update 更新 TokenQuotaPolicy
func (h *Handler) Update(ctx *gin.Context) {
	path := dto.IDReq{}
	if err := ctx.ShouldBindUri(&path); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := path.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	request := dto.UpdateTokenQuotaPolicyReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	policyID := path.ID
	if err := h.service.Update(ctx.Request.Context(), policyID, request.Version, request.Spec()); err != nil {
		h.logger.Error("update token quota policy failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新 Token 配额策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateTokenQuotaPolicyResp{Success: true})
}

// SetEnabled 设置 TokenQuotaPolicy 启用状态
func (h *Handler) SetEnabled(ctx *gin.Context) {
	path := dto.IDReq{}
	if err := ctx.ShouldBindUri(&path); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := path.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	request := dto.SetEnabledReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	policyID := path.ID
	enabled := request.Value()
	if err := h.service.SetEnabled(ctx.Request.Context(), policyID, enabled); err != nil {
		h.logger.Error("set token quota policy enabled failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "enabled", enabled, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "设置 Token 配额策略状态失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.SetEnabledResp{Success: true})
}

// Delete 删除 TokenQuotaPolicy
func (h *Handler) Delete(ctx *gin.Context) {
	path := dto.IDReq{}
	if err := ctx.ShouldBindUri(&path); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := path.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	policyID := path.ID
	if err := h.service.Delete(ctx.Request.Context(), policyID); err != nil {
		h.logger.Error("delete token quota policy failed", "request_id", ctx.GetString(requestid.Header), "policy_id", policyID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除 Token 配额策略失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteTokenQuotaPolicyResp{Success: true})
}
