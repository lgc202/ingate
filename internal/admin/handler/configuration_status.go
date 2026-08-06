package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/admin/dto/configurationstatus"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// GetConfigurationStatus 返回全部声明式资源的配置状态
func (h *Handler) GetConfigurationStatus(ctx *gin.Context) {
	report, err := h.services.ConfigurationStatus.Get(ctx.Request.Context())
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
