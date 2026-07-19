// Package health 提供 ingate-controller 的存活与就绪检查
package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	serverReadHeaderTimeout = 5 * time.Second
	serverShutdownTimeout   = 5 * time.Second
)

// Server 提供 Controller 健康检查 HTTP 接口
type Server struct {
	logger *slog.Logger
	ready  atomic.Bool
}

// NewServer 创建尚未就绪的健康检查服务
func NewServer(logger *slog.Logger) *Server {
	return &Server{logger: logger}
}

// MarkReady 标记 Delivery 已运行且 ADS 与内部 HTTP listener 均已绑定
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
		s.logger.Info("controller health server started", "addr", listener.Addr().String())
		serverErr <- httpServer.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		if err == nil {
			return errors.New("controller health HTTP server stopped unexpectedly")
		}
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve controller health HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
			<-serverErr
			return fmt.Errorf("shut down controller health HTTP: %w", err)
		}
		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve controller health HTTP: %w", err)
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
	return mux
}
