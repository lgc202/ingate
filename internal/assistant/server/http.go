package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	"github.com/lgc202/ingate/internal/assistant/conf"
	conversationservice "github.com/lgc202/ingate/internal/assistant/service/conversation"
	executionservice "github.com/lgc202/ingate/internal/assistant/service/execution"
	modelconfigservice "github.com/lgc202/ingate/internal/assistant/service/modelconfig"
)

// DatabasePinger 定义就绪检查所需的持久存储连通性。
type DatabasePinger interface {
	Ping(context.Context) error
}

// EventStorePinger 定义就绪检查所需的事件存储连通性。
type EventStorePinger interface {
	Ping(context.Context) error
}

type pinger interface {
	Ping(context.Context) error
}

type readinessHandler struct {
	timeout      time.Duration
	dependencies []pinger
}

// NewHTTPServer 创建会话 API、SSE 事件流与健康检查服务。
func NewHTTPServer(
	config *conf.Server,
	conversationAPI *conversationservice.Service,
	executionAPI *executionservice.Service,
	modelAPI *modelconfigservice.Service,
	streamHandler *StreamHandler,
	database DatabasePinger,
	eventStore EventStorePinger,
	logger *slog.Logger,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		// SSE 连接由请求取消和 Stream.read_block 控制，不能套用普通 HTTP 全局超时。
		kratoshttp.Filter(requestIDFilter(), responseNoStoreFilter(), requestBodyLimitFilter(), recoveryFilter(logger)),
		kratoshttp.Middleware(httpMiddleware(logger)...),
		kratoshttp.RequestDecoder(requestDecoder),
		kratoshttp.ResponseEncoder(responseEncoder),
	)
	assistantv1.RegisterConversationServiceHTTPServer(server, conversationAPI)
	assistantv1.RegisterAgentExecutionServiceHTTPServer(server, executionAPI)
	assistantv1.RegisterModelConnectionServiceHTTPServer(server, modelAPI)
	streamHandler.register(server)
	readiness := readinessHandler{
		timeout:      httpConfig.GetReadinessTimeout().AsDuration(),
		dependencies: []pinger{database, eventStore},
	}
	server.HandleFunc("/healthz", live)
	server.HandleFunc("/readyz", readiness.ready)
	return server
}

func live(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *readinessHandler) ready(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()
	for _, dependency := range h.dependencies {
		if err := dependency.Ping(ctx); err != nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(response http.ResponseWriter, statusCode int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(statusCode)
	_ = json.NewEncoder(response).Encode(value)
}
