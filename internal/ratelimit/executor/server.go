// Package executor 实现内置全局限流执行器
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/lgc202/ingate/internal/ratelimit/protocol"
)

const (
	defaultReadHeaderTimeout = 5 * time.Second
	defaultCommandTimeout    = 50 * time.Millisecond
)

// Server 承载执行器 HTTP API
type Server struct {
	logger  *slog.Logger
	clients *ClientManager
}

// NewServer 创建执行器 HTTP 服务
func NewServer(logger *slog.Logger) *Server {
	return &Server{
		logger:  logger,
		clients: NewClientManager(),
	}
}

// Handler 返回执行器 HTTP 路由
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST /v1/rate-limit/check", s.check)
	return mux
}

// HTTPServer 创建标准库 HTTP server
func (s *Server) HTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) check(w http.ResponseWriter, r *http.Request) {
	var request protocol.CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if request.SchemaVersion == "" {
		request.SchemaVersion = protocol.SchemaVersionV1
	}
	if request.SchemaVersion != protocol.SchemaVersionV1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported schema version"})
		return
	}
	if len(request.Checks) == 0 {
		writeJSON(w, http.StatusOK, protocol.CheckResponse{SchemaVersion: protocol.SchemaVersionV1})
		return
	}

	results := make([]protocol.Result, 0, len(request.Checks))
	for _, check := range request.Checks {
		result, err := s.executeCheck(r.Context(), check)
		if err != nil {
			s.logger.Error("rate limit check failed", "policy", check.PolicyName, "rule", check.RuleName, "redis_store", check.RedisStore.ID, "err", err)
			result = protocol.Result{
				PolicyName: check.PolicyName,
				RuleName:   check.RuleName,
				Allowed:    false,
				Error:      err.Error(),
			}
		}
		results = append(results, result)
	}

	writeJSON(w, http.StatusOK, protocol.CheckResponse{
		SchemaVersion: protocol.SchemaVersionV1,
		Results:       results,
	})
}

func (s *Server) executeCheck(ctx context.Context, check protocol.Check) (protocol.Result, error) {
	if check.RedisKey == "" {
		return protocol.Result{}, errors.New("redis key is required")
	}
	if check.Limit.Requests <= 0 || check.Limit.WindowSeconds <= 0 {
		return protocol.Result{}, errors.New("limit requests and windowSeconds must be greater than zero")
	}

	timeout := defaultCommandTimeout
	if check.TimeoutMillis > 0 {
		timeout = time.Duration(check.TimeoutMillis) * time.Millisecond
	} else if check.RedisStore.CommandTimeoutMillis > 0 {
		timeout = time.Duration(check.RedisStore.CommandTimeoutMillis) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := s.clients.Client(check.RedisStore)
	if err != nil {
		return protocol.Result{}, err
	}
	switch check.Algorithm {
	case "", protocol.AlgorithmFixedWindow:
		return fixedWindow(ctx, client, check)
	case protocol.AlgorithmSlidingWindow:
		return slidingWindow(ctx, client, check)
	case protocol.AlgorithmTokenBucket:
		return tokenBucket(ctx, client, check)
	default:
		return protocol.Result{}, errors.New("unsupported rate limit algorithm")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
