// Package upstreamcredential 提供上游访问凭据控制台 HTTP 接口
package upstreamcredential

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/adminapi/dto/upstreamcredential"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	credentialservice "github.com/lgc202/ingate/internal/adminapi/service/upstreamcredential"
)

// Handler 处理 UpstreamCredential HTTP 请求
type Handler struct {
	service *credentialservice.Service
	logger  *slog.Logger
}

// New 创建 UpstreamCredential handler
func New(service *credentialservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List 返回 UpstreamCredential 列表
func (h *Handler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list upstream credentials failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询访问凭据列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListUpstreamCredentialsResp(result))
}

// Get 返回单个 UpstreamCredential
func (h *Handler) Get(ctx *gin.Context) {
	request := dto.IDReq{}
	if err := ctx.ShouldBindUri(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	result, err := h.service.Get(ctx.Request.Context(), request.ID)
	if err != nil {
		h.logger.Error("get upstream credential failed", "request_id", ctx.GetString(requestid.Header), "credential_id", request.ID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询访问凭据失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetUpstreamCredentialResp(result))
}

// Create 创建 UpstreamCredential
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreateUpstreamCredentialReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	credentialID, err := h.service.Create(ctx.Request.Context(), request.Params())
	if err != nil {
		h.logger.Error("create upstream credential failed", "request_id", ctx.GetString(requestid.Header), "name", request.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建访问凭据失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateUpstreamCredentialResp{Success: true, ID: credentialID})
}

// Update 更新 UpstreamCredential
func (h *Handler) Update(ctx *gin.Context) {
	request := dto.UpdateUpstreamCredentialReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	path := dto.IDReq{}
	if err := ctx.ShouldBindUri(&path); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := path.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	if err := h.service.Update(ctx.Request.Context(), path.ID, request.Params()); err != nil {
		h.logger.Error("update upstream credential failed", "request_id", ctx.GetString(requestid.Header), "credential_id", path.ID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新访问凭据失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateUpstreamCredentialResp{Success: true})
}

// Delete 删除 UpstreamCredential
func (h *Handler) Delete(ctx *gin.Context) {
	request := dto.IDReq{}
	if err := ctx.ShouldBindUri(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	if err := h.service.Delete(ctx.Request.Context(), request.ID); err != nil {
		h.logger.Error("delete upstream credential failed", "request_id", ctx.GetString(requestid.Header), "credential_id", request.ID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除访问凭据失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteUpstreamCredentialResp{Success: true})
}
