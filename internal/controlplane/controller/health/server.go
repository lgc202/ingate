package health

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const healthStatusOK = "ok"

const shutdownTimeout = 10 * time.Second

type Server struct {
	httpServer *http.Server
}

func NewServer(bindAddress string) (*Server, error) {
	if bindAddress == "" {
		return nil, fmt.Errorf("healthz bind address must not be empty")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", okHandler)
	mux.HandleFunc("/readyz", okHandler)

	return &Server{
		httpServer: &http.Server{
			Addr:    bindAddress,
			Handler: mux,
		},
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return fmt.Errorf("health server is not initialized")
	}

	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return err
	}

	return s.RunWithListener(ctx, listener)
}

func (s *Server) RunWithListener(ctx context.Context, listener net.Listener) error {
	if s == nil || s.httpServer == nil {
		return fmt.Errorf("health server is not initialized")
	}
	if listener == nil {
		return fmt.Errorf("listener must not be nil")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.Serve(listener)
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
		return nil
	}
}

func (s *Server) Handler() http.Handler {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.Handler
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(healthStatusOK))
}
