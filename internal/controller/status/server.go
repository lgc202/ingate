package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/lgc202/ingate/internal/envoy/config"
	"github.com/lgc202/ingate/internal/envoy/delivery"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	serverReadHeaderTimeout = 5 * time.Second
	serverShutdownTimeout   = 5 * time.Second
)

type diagnostic struct {
	Severity config.Severity `json:"severity"`
	Kind     gatewayv1.Kind  `json:"kind"`
	ID       string          `json:"id"`
	Reason   config.Reason   `json:"reason"`
	Message  string          `json:"message"`
}

type response struct {
	delivery.Status
	Reconciled  bool         `json:"reconciled"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

// Server 提供 Controller 健康检查和内部运行状态 HTTP 接口
type Server struct {
	runtime  *Runtime
	delivery *delivery.Delivery
	logger   *slog.Logger
	ready    atomic.Bool
}

// NewServer 创建尚未就绪的内部状态服务
func NewServer(runtime *Runtime, configDelivery *delivery.Delivery, logger *slog.Logger) *Server {
	return &Server{
		runtime:  runtime,
		delivery: configDelivery,
		logger:   logger,
	}
}

// MarkReady 标记 Last Good/Baseline 已恢复且 ADS listener 已绑定
func (s *Server) MarkReady() {
	s.ready.Store(true)
}

// Serve 在已经绑定的 listener 上运行内部 HTTP 服务
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	httpServer := &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: serverReadHeaderTimeout,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	serverErr := make(chan error, 1)
	go func() {
		s.logger.Info("controller internal http server started", "addr", listener.Addr().String())
		serverErr <- httpServer.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		if err == nil {
			return errors.New("controller internal HTTP server stopped unexpectedly")
		}
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve controller internal HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
			<-serverErr
			return fmt.Errorf("shut down controller internal HTTP: %w", err)
		}
		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve controller internal HTTP: %w", err)
		}
		return nil
	}
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, _ *http.Request) {
		if !s.ready.Load() {
			http.Error(writer, "not ready", http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /internal/v1/status", s.writeStatus)
	return mux
}

func (s *Server) writeStatus(writer http.ResponseWriter, _ *http.Request) {
	runtimeState := s.runtime.Snapshot()
	diagnostics := make([]diagnostic, len(runtimeState.Diagnostics))
	for index, value := range runtimeState.Diagnostics {
		diagnostics[index] = diagnostic{
			Severity: value.Severity,
			Kind:     value.Kind,
			ID:       value.ID,
			Reason:   value.Reason,
			Message:  value.Message,
		}
	}

	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(response{
		Status:      s.delivery.Status(),
		Reconciled:  runtimeState.Reconciled,
		Diagnostics: diagnostics,
	}); err != nil {
		s.logger.Warn("write controller status response", "err", err)
	}
}
