package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler"
	"github.com/lgc202/ingate/internal/adminapi/service"
	"github.com/lgc202/ingate/internal/adminapi/store"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Server 提供面向控制台的管理 API 服务生命周期
type Server struct {
	client        clientset.Interface
	listenAddress string
	consoleDir    string
	logger        *slog.Logger
}

// New 创建管理 API 服务
func New(client clientset.Interface, listenAddress, consoleDir string, logger *slog.Logger) *Server {
	return &Server{
		client:        client,
		listenAddress: listenAddress,
		consoleDir:    consoleDir,
		logger:        logger,
	}
}

// Run 启动 HTTP 服务
func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		return err
	}
	defer listener.Close()

	httpServer := &http.Server{Handler: s.router()}
	serverErr := make(chan error, 1)
	go func() {
		s.logger.Info("admin api http server started", "addr", listener.Addr().String())
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
	resourceStore := store.New(s.client)
	resourceService := service.New(resourceStore)
	return handler.New(resourceService, s.logger)
}

func init() {
	gin.SetMode(gin.ReleaseMode)
}
