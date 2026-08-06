package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/admin/dto/accesskey"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// ListAccessKey 返回访问密钥列表
func (h *Handler) ListAccessKey(ctx *gin.Context) {
	keys, err := h.services.AccessKey.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list access keys failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询访问密钥列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListAccessKeysResp(keys))
}

// CreateAccessKey 创建访问密钥
func (h *Handler) CreateAccessKey(ctx *gin.Context) {
	request := dto.CreateAccessKeyReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	key, secret, err := h.services.AccessKey.Create(
		ctx.Request.Context(),
		request.Name,
		request.AllowedModels,
		request.ExpiresAt,
	)
	if err != nil {
		h.logger.Error("create access key failed", "request_id", ctx.GetString(requestid.Header), "name", request.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建访问密钥失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewCreateAccessKeyResp(key, secret))
}

// UpdateAccessKey 更新访问密钥
func (h *Handler) UpdateAccessKey(ctx *gin.Context) {
	path := dto.IDReq{}
	if err := ctx.ShouldBindUri(&path); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := path.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	request := dto.UpdateAccessKeyReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	key, err := h.services.AccessKey.Update(
		ctx.Request.Context(),
		path.ID,
		request.Name,
		request.AllowedModels,
		request.ExpiresAt,
	)
	if err != nil {
		h.logger.Error("update access key failed", "request_id", ctx.GetString(requestid.Header), "access_key_id", path.ID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新访问密钥失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewUpdateAccessKeyResp(key))
}

// SetAccessKeyEnabled 设置访问密钥启用状态
func (h *Handler) SetAccessKeyEnabled(ctx *gin.Context) {
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

	enabled := request.Value()
	key, err := h.services.AccessKey.SetEnabled(ctx.Request.Context(), path.ID, enabled)
	if err != nil {
		h.logger.Error("set access key enabled failed", "request_id", ctx.GetString(requestid.Header), "access_key_id", path.ID, "enabled", enabled, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "设置访问密钥状态失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewUpdateAccessKeyResp(key))
}

// RotateAccessKey 轮换访问密钥
func (h *Handler) RotateAccessKey(ctx *gin.Context) {
	path := dto.IDReq{}
	if err := ctx.ShouldBindUri(&path); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := path.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	key, secret, err := h.services.AccessKey.Rotate(ctx.Request.Context(), path.ID)
	if err != nil {
		h.logger.Error("rotate access key failed", "request_id", ctx.GetString(requestid.Header), "access_key_id", path.ID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "轮换访问密钥失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewRotateAccessKeyResp(key, secret))
}

// DeleteAccessKey 删除访问密钥
func (h *Handler) DeleteAccessKey(ctx *gin.Context) {
	path := dto.IDReq{}
	if err := ctx.ShouldBindUri(&path); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := path.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	if err := h.services.AccessKey.Delete(ctx.Request.Context(), path.ID); err != nil {
		h.logger.Error("delete access key failed", "request_id", ctx.GetString(requestid.Header), "access_key_id", path.ID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除访问密钥失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteAccessKeyResp{Success: true})
}
