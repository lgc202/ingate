// Package systemstatus 处理单配置域运行状态 HTTP 请求
package systemstatus

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/systemstatus/dto"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	systemstatusservice "github.com/lgc202/ingate/internal/adminapi/service/systemstatus"
)

// Handler 处理单配置域运行状态请求
type Handler struct {
	service *systemstatusservice.Service
	logger  *slog.Logger
}

// New 创建单配置域运行状态 handler
func New(service *systemstatusservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// Get 返回 Controller 和 Envoy 的实时运行状态
func (h *Handler) Get(ctx *gin.Context) {
	result, err := h.service.Get(ctx.Request.Context())
	if err != nil {
		h.logger.Error("get system status failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewUnavailableSystemStatusResp())
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetSystemStatusResp(result))
}
