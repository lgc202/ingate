// Package certificate 提供 Certificate 控制台 HTTP 接口
package certificate

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/admin/dto/certificate"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	certificateservice "github.com/lgc202/ingate/internal/admin/service/certificate"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// Handler 处理 Certificate HTTP 请求
type Handler struct {
	service *certificateservice.Service
	logger  *slog.Logger
}

// New 创建 Certificate handler
func New(service *certificateservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List 返回 Certificate 列表
func (h *Handler) List(ctx *gin.Context) {
	certificates, err := h.service.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list certificates failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询证书列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListCertificatesResp(certificates))
}

// Get 返回单个 Certificate
func (h *Handler) Get(ctx *gin.Context) {
	certificateID := ctx.Param("id")
	certificate, err := h.service.Get(ctx.Request.Context(), certificateID)
	if err != nil {
		h.logger.Error("get certificate failed", "request_id", ctx.GetString(requestid.Header), "certificate_id", certificateID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询证书失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetCertificateResp(certificate))
}

// Create 创建 Certificate
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreateCertificateReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	certificateID, err := h.service.Create(ctx.Request.Context(), request.Spec())
	if err != nil {
		h.logger.Error("create certificate failed", "request_id", ctx.GetString(requestid.Header), "name", request.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建证书失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateCertificateResp{Success: true, ID: certificateID})
}

// Update 更新 Certificate
func (h *Handler) Update(ctx *gin.Context) {
	request := dto.UpdateCertificateReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	certificateID := ctx.Param("id")
	if err := h.service.Update(ctx.Request.Context(), certificateID, request.Version, request.Spec()); err != nil {
		h.logger.Error("update certificate failed", "request_id", ctx.GetString(requestid.Header), "certificate_id", certificateID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新证书失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateCertificateResp{Success: true})
}

// Delete 删除 Certificate
func (h *Handler) Delete(ctx *gin.Context) {
	certificateID := ctx.Param("id")
	if err := h.service.Delete(ctx.Request.Context(), certificateID); err != nil {
		h.logger.Error("delete certificate failed", "request_id", ctx.GetString(requestid.Header), "certificate_id", certificateID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除证书失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteCertificateResp{Success: true})
}
