package redisstore

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler/redisstore/dto"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	redisstoreservice "github.com/lgc202/ingate/internal/adminapi/service/redisstore"
)

// Handler 处理 RedisStore HTTP 请求
type Handler struct {
	service *redisstoreservice.Service
	logger  *slog.Logger
}

// New 创建 RedisStore handler
func New(service *redisstoreservice.Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// List 返回 RedisStore 列表
func (h *Handler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx.Request.Context())
	if err != nil {
		h.logger.Error("list redis stores failed", "request_id", ctx.GetString(requestid.Header), "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询 Redis 配置列表失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewListRedisStoresResp(result))
}

// Get 返回单个 RedisStore
func (h *Handler) Get(ctx *gin.Context) {
	redisStoreID := ctx.Param("id")
	result, err := h.service.Get(ctx.Request.Context(), redisStoreID)
	if err != nil {
		h.logger.Error("get redis store failed", "request_id", ctx.GetString(requestid.Header), "redis_store_id", redisStoreID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "查询 Redis 配置失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.NewGetRedisStoreResp(result))
}

// Create 创建 RedisStore
func (h *Handler) Create(ctx *gin.Context) {
	request := dto.CreateRedisStoreReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	redisStoreID, err := h.service.Create(ctx.Request.Context(), h.createParams(request))
	if err != nil {
		h.logger.Error("create redis store failed", "request_id", ctx.GetString(requestid.Header), "name", request.Name, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "创建 Redis 配置失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.CreateRedisStoreResp{Success: true, ID: redisStoreID})
}

// Update 更新 RedisStore
func (h *Handler) Update(ctx *gin.Context) {
	request := dto.UpdateRedisStoreReq{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := request.Validate(); err != nil {
		response.GinAbortJSONResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}

	redisStoreID := ctx.Param("id")
	if err := h.service.Update(ctx.Request.Context(), redisStoreID, h.updateParams(request)); err != nil {
		h.logger.Error("update redis store failed", "request_id", ctx.GetString(requestid.Header), "redis_store_id", redisStoreID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "更新 Redis 配置失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.UpdateRedisStoreResp{Success: true})
}

// Delete 删除 RedisStore
func (h *Handler) Delete(ctx *gin.Context) {
	redisStoreID := ctx.Param("id")
	if err := h.service.Delete(ctx.Request.Context(), redisStoreID); err != nil {
		h.logger.Error("delete redis store failed", "request_id", ctx.GetString(requestid.Header), "redis_store_id", redisStoreID, "err", err)
		if userError, ok := errors.AsType[*xerrors.UserError](err); ok {
			response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, userError.Error(), nil)
			return
		}
		response.GinAbortJSONResponse(ctx, http.StatusInternalServerError, "删除 Redis 配置失败", nil)
		return
	}
	response.GinJSONResponse(ctx, http.StatusOK, "ok", dto.DeleteRedisStoreResp{Success: true})
}

func (h *Handler) createParams(request dto.CreateRedisStoreReq) redisstoreservice.CreateRedisStoreParams {
	return redisstoreservice.CreateRedisStoreParams{RedisStoreParams: h.redisStoreParams(request.RedisStoreConfig)}
}

func (h *Handler) updateParams(request dto.UpdateRedisStoreReq) redisstoreservice.UpdateRedisStoreParams {
	return redisstoreservice.UpdateRedisStoreParams{
		Version:          request.Version,
		RedisStoreParams: h.redisStoreParams(request.RedisStoreConfig),
	}
}

func (h *Handler) redisStoreParams(config dto.RedisStoreConfig) redisstoreservice.RedisStoreParams {
	return redisstoreservice.RedisStoreParams{
		Name:                 config.Name,
		Description:          config.Description,
		Mode:                 config.Mode,
		Address:              config.Address,
		Addresses:            config.Addresses,
		DB:                   config.DB,
		TLS:                  config.TLS,
		TLSServerName:        config.TLSServerName,
		Username:             config.Username,
		PasswordRef:          config.PasswordRef,
		ConnectTimeoutMillis: config.ConnectTimeoutMillis,
		CommandTimeoutMillis: config.CommandTimeoutMillis,
		PoolSize:             config.PoolSize,
		MinIdleConns:         config.MinIdleConns,
		SentinelMaster:       config.SentinelMaster,
	}
}
