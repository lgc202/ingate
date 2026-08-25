// Package server 装配运维助手的 Kratos HTTP transport。
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/assistant/data/mysql"
	redisdata "github.com/lgc202/ingate/internal/assistant/data/redis"
	conversationservice "github.com/lgc202/ingate/internal/assistant/service/conversation"
	modelservice "github.com/lgc202/ingate/internal/assistant/service/model"
)

const forwardedUserHeader = "X-Forwarded-User"

// NewHTTPServer 创建会话 API、SSE 事件流与健康检查服务。
func NewHTTPServer(
	config *conf.Server,
	conversationAPI *conversationservice.Service,
	modelAPI *modelservice.Service,
	streamHandler *streamHandler,
	mysqlStore *mysql.Store,
	eventStore *redisdata.EventStore,
	logger *slog.Logger,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		// SSE 连接由请求取消和 Stream.read_block 控制，不能套用普通 HTTP 全局超时。
		kratoshttp.Filter(requestIDFilter(), recoveryFilter(logger)),
		kratoshttp.Middleware(httpMiddleware(logger)...),
	)
	assistantv1.RegisterConversationServiceHTTPServer(server, conversationAPI)
	assistantv1.RegisterModelConnectionServiceHTTPServer(server, modelAPI)
	streamHandler.register(server)
	health := healthHandler{
		timeout:      httpConfig.GetTimeout().AsDuration(),
		dependencies: []pinger{mysqlStore, eventStore},
	}
	server.HandleFunc("/healthz", health.live)
	server.HandleFunc("/readyz", health.ready)
	return server
}

type pinger interface {
	Ping(context.Context) error
}

type healthHandler struct {
	timeout      time.Duration
	dependencies []pinger
}

func (h *healthHandler) live(response http.ResponseWriter, _ *http.Request) {
	_ = writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *healthHandler) ready(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()
	for _, dependency := range h.dependencies {
		if err := dependency.Ping(ctx); err != nil {
			_ = writeJSON(response, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
	}
	_ = writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(response http.ResponseWriter, status int, value any) error {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(value); err != nil {
		return fmt.Errorf("encode JSON response: %w", err)
	}
	return nil
}
