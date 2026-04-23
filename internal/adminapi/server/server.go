package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminconfig "github.com/lgc202/ingate/internal/adminapi/config"
	"github.com/lgc202/ingate/internal/adminapi/handler"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

const shutdownTimeout = 10 * time.Second

type Server struct {
	httpServer *http.Server
}

func New(cfg adminconfig.Config) (*Server, error) {
	apiStore, err := store.NewAPIServerStore(cfg)
	if err != nil {
		return nil, err
	}

	gin.SetMode(gin.ReleaseMode)
	router := newRouter(handlers{
		health:        handler.NewHealthHandler(),
		gateway:       handler.NewGatewayHandler(apiStore),
		backend:       handler.NewBackendHandler(apiStore),
		route:         handler.NewRouteHandler(apiStore),
		certificate:   handler.NewCertificateHandler(biz.NewCertificateService(apiStore)),
		authPolicy:    handler.NewAuthPolicyHandler(apiStore),
		trafficPolicy: handler.NewTrafficPolicyHandler(apiStore),
		overview:      handler.NewOverviewHandler(biz.NewOverviewService(apiStore)),
		topology:      handler.NewTopologyHandler(biz.NewTopologyService(apiStore)),
		event:         handler.NewEventHandler(biz.NewEventService(apiStore)),
	}, cfg.AdminToken)

	return &Server{httpServer: &http.Server{
		Addr:    cfg.ListenAddress(),
		Handler: router,
	}}, nil
}

func (s *Server) Run(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return fmt.Errorf("admin-api server is not initialized")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	}
}
