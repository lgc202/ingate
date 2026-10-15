package modelproxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestProxyUsesRestrictedSocket(t *testing.T) {
	forbiddenHeaders := []string{
		"Authorization", "Cookie", "X-Api-Key", "Forwarded", "X-Forwarded-For",
		"X-Envoy-Upstream-Rq-Timeout-Ms", "X-Ai-Eg-Model", "X-Custom-Secret",
	}
	socket := serveUnix(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/custom%20base/chat/completions" || r.URL.RawQuery != "" {
			t.Errorf("egress request = %s %s, want POST selected operation without client query", r.Method, r.URL)
		}
		if r.Host != "ingate-ai-egress" || r.Header.Get(aidataplane.TargetHeader) != "selected" {
			t.Errorf("egress Host/target = %q/%q, want fixed Host and selected target", r.Host, r.Header.Get(aidataplane.TargetHeader))
		}
		for _, header := range forbiddenHeaders {
			if r.Header.Get(header) != "" {
				t.Errorf("egress retained forbidden client header %q", header)
			}
		}
		if r.Header.Get("Accept") != "application/json" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("egress body negotiation = %v, want JSON", r.Header)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != `{"model":"prepared"}` {
			t.Errorf("egress body/error = %s/%v, want prepared body", body, err)
		}
		w.Header().Set("X-Provider", "preserved")
		w.WriteHeader(http.StatusTooManyRequests)
		if _, err := io.WriteString(w, `{"error":{"message":"provider limit"}}`); err != nil {
			t.Errorf("WriteString(provider response) error = %v", err)
		}
	}))
	p := testProxy(t, socket)
	request := httptest.NewRequest(
		http.MethodPost,
		"http://untrusted.invalid/v1/chat/completions?key=secret",
		strings.NewReader(`{"model":"prepared"}`),
	)
	request = request.WithContext(WithBackend(t.Context(), aiconfig.ModelBackend{
		TargetName: "selected", Schema: aidataplane.APISchema{Prefix: "/custom%20base/"},
	}))
	for _, header := range forbiddenHeaders {
		request.Header.Set(header, "spoofed")
	}
	request.Header.Set(aidataplane.TargetHeader, "spoofed")
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()

	p.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Errorf("ServeHTTP(provider error) status = %d, want 429", response.Code)
	}
	if response.Body.String() != `{"error":{"message":"provider limit"}}` {
		t.Errorf("ServeHTTP(provider error) body = %s, want unchanged provider error", response.Body)
	}
	if response.Header().Get("X-Provider") != "preserved" {
		t.Errorf("ServeHTTP(provider error) headers = %v, want preserved provider header", response.Header())
	}
	if request.URL.Host != "untrusted.invalid" || request.Header.Get(aidataplane.TargetHeader) != "spoofed" {
		t.Errorf("ServeHTTP() mutated original request: %s, %v", request.URL, request.Header)
	}
}

func TestProxyRejectsWithoutDial(t *testing.T) {
	for _, tt := range []struct {
		name    string
		method  string
		path    string
		backend aiconfig.ModelBackend
	}{
		{name: "no target", method: http.MethodPost},
		{name: "other method", method: http.MethodGet, backend: aiconfig.ModelBackend{TargetName: "selected"}},
		{name: "other operation", method: http.MethodPost, path: "/v1/responses", backend: aiconfig.ModelBackend{TargetName: "selected"}},
		{name: "bad prefix", method: http.MethodPost, backend: aiconfig.ModelBackend{TargetName: "selected", Schema: aidataplane.APISchema{Prefix: "%zz"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got error
			p, err := New("/does-not-exist/egress.sock", func(_ http.ResponseWriter, _ *http.Request, err error) { got = err })
			if err != nil {
				t.Fatalf("New(test proxy) error = %v", err)
			}
			t.Cleanup(p.CloseIdleConnections)
			path := tt.path
			if path == "" {
				path = "/v1/chat/completions"
			}
			request := httptest.NewRequest(tt.method, path, nil)
			request = request.WithContext(WithBackend(t.Context(), tt.backend))

			p.ServeHTTP(httptest.NewRecorder(), request)

			if !errors.Is(got, ErrTargetUnavailable) {
				t.Errorf("ServeHTTP(%s) error = %v, want target rejection before socket dial", tt.name, got)
			}
		})
	}
}

func TestProxyStreamsAndPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ended := make(chan struct{})
	socket := serveUnix(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(ended)
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := io.WriteString(w, "data: first\n\n"); err != nil {
			t.Errorf("WriteString(first SSE event) error = %v", err)
			return
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush(first SSE event) error = %v", err)
			return
		}
		<-r.Context().Done()
	}))
	p := testProxy(t, socket)
	// 入口 Server 在测试中才设置可信绑定；不把它作为生产身份校验。
	entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.ServeHTTP(w, r.WithContext(WithBackend(r.Context(), aiconfig.ModelBackend{
			TargetName: "selected", Schema: aidataplane.APISchema{Prefix: "/v1"},
		})))
	}))
	t.Cleanup(entry.Close)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, entry.URL+"/v1/chat/completions", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("NewRequestWithContext(stream) error = %v", err)
	}

	response, err := entry.Client().Do(request)
	if err != nil {
		t.Fatalf("Do(stream) error = %v", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("Close(stream response) error = %v", err)
		}
	}()
	first, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil || first != "data: first\n" {
		t.Fatalf("ReadString(first SSE event) = (%q, %v), want event before stream completion", first, err)
	}
	cancel()

	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Error("client cancellation did not reach the simulated Unix-socket peer")
	}
}

func TestNewRejectsInvalidOptions(t *testing.T) {
	for _, socket := range []string{"", "relative.sock", "/tmp/invalid\x00.sock"} {
		p, err := New(socket, func(http.ResponseWriter, *http.Request, error) {})

		if p != nil || err == nil {
			t.Errorf("New(socket %q) = (%v, %v), want nil proxy and error", socket, p, err)
		}
	}
	p, err := New("/tmp/egress.sock", nil)

	if p != nil || err == nil {
		t.Errorf("New(nil error handler) = (%v, %v), want nil proxy and error", p, err)
	}
}

func TestProxySocketFailureHasNoNetworkFallback(t *testing.T) {
	var got error
	p, err := New("/does-not-exist/egress.sock", func(_ http.ResponseWriter, _ *http.Request, err error) { got = err })
	if err != nil {
		t.Fatalf("New(missing socket) error = %v", err)
	}
	t.Cleanup(p.CloseIdleConnections)
	request := httptest.NewRequest(http.MethodPost, "http://untrusted.invalid/v1/chat/completions", nil)
	request = request.WithContext(WithBackend(t.Context(), aiconfig.ModelBackend{TargetName: "selected", Schema: aidataplane.APISchema{Prefix: "/v1"}}))

	p.ServeHTTP(httptest.NewRecorder(), request)

	if networkError, ok := errors.AsType[*net.OpError](got); !ok || networkError.Net != "unix" {
		t.Errorf("ServeHTTP(missing socket) error = %v, want Unix dial failure without TCP fallback", got)
	}
}

func testProxy(t *testing.T, socket string) *Proxy {
	t.Helper()
	p, err := New(socket, func(w http.ResponseWriter, _ *http.Request, err error) {
		t.Errorf("ServeHTTP(valid target) unexpected proxy error = %v", err)
		http.Error(w, "proxy unavailable", http.StatusBadGateway)
	})
	if err != nil {
		t.Fatalf("New(test proxy) error = %v", err)
	}
	t.Cleanup(p.CloseIdleConnections)

	return p
}

func serveUnix(t *testing.T, handler http.Handler) string {
	t.Helper()
	// t.TempDir 的测试名称可能超过 macOS Unix socket 长度限制，使用短的独立临时目录。
	directory, err := os.MkdirTemp("", "ig-ai-")
	if err != nil {
		t.Fatalf("MkdirTemp(Unix socket) error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Errorf("RemoveAll(test socket directory) error = %v", err)
		}
	})
	socket := filepath.Join(directory, "egress.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("Listen(test Unix socket) error = %v", err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: handler}}
	server.Start()
	t.Cleanup(server.Close)

	return socket
}
