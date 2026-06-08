// Package server 承载 ingate-dataplane 的服务生命周期
package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/dataplane/handler"
	"github.com/lgc202/ingate/internal/dataplane/service"
)

const defaultReadHeaderTimeout = 5 * time.Second

// Server 提供 ingate-dataplane HTTP 服务
type Server struct {
	listenAddress string
	logger        *slog.Logger
}

// New 创建 ingate-dataplane 服务
func New(listenAddress string, logger *slog.Logger) *Server {
	return &Server{
		listenAddress: listenAddress,
		logger:        logger,
	}
}

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// Run 启动 HTTP 服务
func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		return err
	}
	defer listener.Close()

	httpServer := &http.Server{
		Handler:           s.router(),
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}
	serverErr := make(chan error, 1)
	go func() {
		s.logger.Info("dataplane http server started", "addr", listener.Addr().String())
		serverErr <- httpServer.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		if err := httpServer.Shutdown(context.Background()); err != nil {
			return err
		}
		return ctx.Err()
	case err := <-serverErr:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) newHandler() *handler.Handler {
	return handler.New(service.New(s.logger))
}
