package policybinding

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	dto "github.com/lgc202/ingate/internal/adminapi/dto/policybinding"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	policybindingservice "github.com/lgc202/ingate/internal/adminapi/service/policybinding"
)

// Handler 处理 PolicyBinding HTTP 请求
type Handler struct {
	service *policybindingservice.Service
	logger  *slog.Logger
}

// New 创建 PolicyBinding handler
func New(service *policybindingservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List 返回 PolicyBinding 列表
func (h *Handler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list policy bindings failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询策略绑定列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListPolicyBindingsResp(result))
}

// Get 返回单个 PolicyBinding
func (h *Handler) Get(ctx *gin.Context) {
	bindingID := ctx.Param("id")
	result, err := h.service.Get(ctx.Request.Context(), bindingID)
	if err != nil {
		h.logger.Error("get policy binding failed", "request_id", ctx.GetString(requestid.Header), "binding_id", bindingID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询策略绑定失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetPolicyBindingResp(result))
}

// Create 创建 PolicyBinding
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreatePolicyBindingReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	bindingID, err := h.service.Create(ctx.Request.Context(), h.createParams(request))
	if err != nil {
		h.logger.Error("create policy binding failed", "request_id", ctx.GetString(requestid.Header), "name", request.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建策略绑定失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreatePolicyBindingResp{Success: true, ID: bindingID})
}

// Update 更新 PolicyBinding
func (h *Handler) Update(ctx *gin.Context) {
	request := dto.UpdatePolicyBindingReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	bindingID := ctx.Param("id")
	if err := h.service.Update(ctx.Request.Context(), bindingID, h.updateParams(request)); err != nil {
		h.logger.Error("update policy binding failed", "request_id", ctx.GetString(requestid.Header), "binding_id", bindingID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新策略绑定失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdatePolicyBindingResp{Success: true})
}

// SetEnabled 设置 PolicyBinding 启用状态
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

	bindingID := ctx.Param("id")
	if err := h.service.SetEnabled(ctx.Request.Context(), bindingID, request.Enabled); err != nil {
		h.logger.Error("set policy binding enabled failed", "request_id", ctx.GetString(requestid.Header), "binding_id", bindingID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "设置策略绑定状态失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.SetEnabledResp{Success: true})
}

// Delete 删除 PolicyBinding
func (h *Handler) Delete(ctx *gin.Context) {
	bindingID := ctx.Param("id")
	if err := h.service.Delete(ctx.Request.Context(), bindingID); err != nil {
		h.logger.Error("delete policy binding failed", "request_id", ctx.GetString(requestid.Header), "binding_id", bindingID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除策略绑定失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeletePolicyBindingResp{Success: true})
}

func (h *Handler) createParams(request dto.CreatePolicyBindingReq) policybindingservice.CreateBindingParams {
	return policybindingservice.CreateBindingParams{BindingParams: h.bindingParams(request.PolicyBindingConfig)}
}

func (h *Handler) updateParams(request dto.UpdatePolicyBindingReq) policybindingservice.UpdateBindingParams {
	return policybindingservice.UpdateBindingParams{
		Version:       request.Version,
		BindingParams: h.bindingParams(request.PolicyBindingConfig),
	}
}

func (h *Handler) bindingParams(config dto.PolicyBindingConfig) policybindingservice.BindingParams {
	return policybindingservice.BindingParams{
		Name:        config.Name,
		Description: config.Description,
		Enabled:     config.Enabled,
		TargetRef:   config.TargetRef,
		Policies:    config.Policies,
	}
}
