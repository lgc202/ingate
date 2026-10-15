package envoyclient

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientPinsUnixSocketAndDoesNotRedirect(t *testing.T) {
	var calls atomic.Int32
	socket := serveUnix(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Accept-Encoding") != "" {
			t.Errorf("Do(Unix request) Accept-Encoding = %q, want no automatic negotiation", r.Header.Get("Accept-Encoding"))
		}
		w.Header().Set("Location", "http://another.invalid/redirected")
		w.WriteHeader(http.StatusTemporaryRedirect)
		if _, err := io.WriteString(w, "original redirect body"); err != nil {
			t.Errorf("WriteString(redirect body) error = %v", err)
		}
	}))
	client, err := New(socket)
	if err != nil {
		t.Fatalf("New(valid Unix socket) error = %v", err)
	}
	t.Cleanup(client.CloseIdleConnections)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://not-a-network-destination.invalid/operation", strings.NewReader("prepared"))
	if err != nil {
		t.Fatalf("NewRequestWithContext(Unix request) error = %v", err)
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do(Unix redirect) error = %v", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("Close(redirect response) error = %v", err)
		}
	}()
	body, err := io.ReadAll(response.Body)

	if err != nil || response.StatusCode != http.StatusTemporaryRedirect || string(body) != "original redirect body" || calls.Load() != 1 {
		t.Errorf("Do(Unix redirect) = (status %d, body %q, calls %d, error %v), want unchanged 307 and one call", response.StatusCode, body, calls.Load(), err)
	}
	transport := client.Transport.(*http.Transport)
	if transport.Proxy != nil || !transport.DisableCompression || client.Timeout != 0 {
		t.Errorf("New() transport/timeout = (%+v, %v), want no environment proxy, decompression or overall timeout", transport, client.Timeout)
	}
}

func TestClientSocketFailureHasNoTCPFallback(t *testing.T) {
	client, err := New("/does-not-exist/egress.sock")
	if err != nil {
		t.Fatalf("New(missing socket) error = %v", err)
	}
	t.Cleanup(client.CloseIdleConnections)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://untrusted.invalid/", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext(missing socket) error = %v", err)
	}

	_, err = client.Do(request)

	if networkError, ok := errors.AsType[*net.OpError](err); !ok || networkError.Net != "unix" {
		t.Errorf("Do(missing socket) error = %v, want Unix dial failure without TCP fallback", err)
	}
}

func TestNewRejectsInvalidSocket(t *testing.T) {
	for _, socket := range []string{"", "relative.sock", "/run/invalid\x00.sock"} {
		client, err := New(socket)

		if client != nil || err == nil {
			t.Errorf("New(socket %q) = (%v, %v), want nil and configuration error", socket, client, err)
		}
	}
}

func serveUnix(t *testing.T, handler http.Handler) string {
	t.Helper()
	directory, err := os.MkdirTemp("", "ig-client-")
	if err != nil {
		t.Fatalf("MkdirTemp(Unix peer) error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Errorf("RemoveAll(test socket directory) error = %v", err)
		}
	})
	socket := filepath.Join(directory, "egress.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("Listen(Unix peer) error = %v", err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: handler}}
	server.Start()
	t.Cleanup(server.Close)

	return socket
}
