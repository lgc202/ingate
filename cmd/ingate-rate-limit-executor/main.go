package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/pflag"

	"github.com/lgc202/ingate/internal/ratelimit/executor"
)

const (
	defaultListenAddress    = "127.0.0.1:18081"
	gracefulShutdownTimeout = 5 * time.Second
)

func main() {
	listenAddress := pflag.String("listen-address", defaultListenAddress, "rate limit executor listen address")
	pflag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := executor.NewServer(logger).HTTPServer(*listenAddress)

	go func() {
		logger.Info("starting rate limit executor", "listen_address", *listenAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("rate limit executor stopped unexpectedly", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown rate limit executor failed", "err", err)
		os.Exit(1)
	}
}
