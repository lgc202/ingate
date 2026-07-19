package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
)

func TestRunReturnsNilAfterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	server := New(
		"127.0.0.1:0",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err := server.Run(ctx); err != nil {
		t.Fatalf("Server.Run() error = %v, want nil", err)
	}
}
