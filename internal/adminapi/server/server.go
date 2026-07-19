package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

const (
	serverReadHeaderTimeout = 5 * time.Second
	serverShutdownTimeout   = 5 * time.Second
)

// Server 提供面向控制台的管理 API 服务生命周期
type Server struct {
	listenAddress string
	handler       http.Handler
	logger        *slog.Logger
}

// New 创建管理 API 服务
func New(
	listenAddress string,
	handler http.Handler,
	logger *slog.Logger,
) *Server {
	return &Server{
		listenAddress: listenAddress,
		handler:       handler,
		logger:        logger,
	}
}

// Run 启动 HTTP 服务
func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		return fmt.Errorf("listen for admin API HTTP on %q: %w", s.listenAddress, err)
	}

	httpServer := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	serverErr := make(chan error, 1)
	go func() {
		s.logger.Info("admin api http server started", "addr", listener.Addr().String())
		serverErr <- httpServer.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve admin API HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			shutdownErr := fmt.Errorf("shut down admin API HTTP: %w", err)
			if closeErr := httpServer.Close(); closeErr != nil {
				shutdownErr = errors.Join(shutdownErr, fmt.Errorf("force close admin API HTTP: %w", closeErr))
			}
			if serveErr := <-serverErr; serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				shutdownErr = errors.Join(shutdownErr, fmt.Errorf("serve admin API HTTP: %w", serveErr))
			}
			return shutdownErr
		}
		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve admin API HTTP: %w", err)
		}
		return nil
	}
}
