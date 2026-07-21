// Package configurationstatus 处理配置状态聚合 HTTP 请求
package configurationstatus

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/adminapi/dto/configurationstatus"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	configurationstatusservice "github.com/lgc202/ingate/internal/adminapi/service/configurationstatus"
)

// Handler 处理配置状态聚合 HTTP 请求
type Handler struct {
	service *configurationstatusservice.Service
	logger  *slog.Logger
}

// New 创建配置状态聚合 handler
func New(service *configurationstatusservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// Get 返回全部声明式资源的配置状态
func (h *Handler) Get(ctx *gin.Context) {
	report, err := h.service.Get(ctx.Request.Context())
	if err != nil {
		h.logger.Error("get configuration status failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询配置状态失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetConfigurationStatusResp(report))
}
