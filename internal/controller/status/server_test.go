package status

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerOnlyExposesHealthAndReadiness(t *testing.T) {
	server := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)))

	assertHTTPStatus(t, server.handler(), http.MethodGet, "/healthz", http.StatusOK)
	assertHTTPStatus(t, server.handler(), http.MethodGet, "/readyz", http.StatusServiceUnavailable)
	assertHTTPStatus(t, server.handler(), http.MethodGet, "/internal/v1/status", http.StatusNotFound)

	server.MarkReady()
	assertHTTPStatus(t, server.handler(), http.MethodGet, "/readyz", http.StatusOK)
}

func assertHTTPStatus(t *testing.T, handler http.Handler, method, target string, want int) {
	t.Helper()
	request := httptest.NewRequest(method, target, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != want {
		t.Errorf("ServeHTTP(%s %s) status = %d, want %d", method, target, response.Code, want)
	}
}
