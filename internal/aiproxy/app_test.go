package aiproxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/apischema/openai"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/envoyclient"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestNewHTTPServerConnectsModelPipeline(t *testing.T) {
	previousMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()
	t.Cleanup(func() { http.DefaultServeMux = previousMux })
	http.HandleFunc("/unintended-debug/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	var calls atomic.Int32
	egress := unixListener(t, "egress.sock")
	peerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll(model pipeline) error = %v", err)
			return
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil {
			t.Errorf("Unmarshal(model pipeline) error = %v", err)
			return
		}

		wantFields := map[string]json.RawMessage{"model": json.RawMessage(`"code-v2"`), "messages": json.RawMessage(`[]`)}
		if diff := cmp.Diff(wantFields, fields); diff != "" {
			t.Errorf("model pipeline body mismatch (-want +got):\n%s", diff)
		}

		const target = "ai/target/96914ad662f712bc8376b8a6ec1aee8de3bb4f51dcd55f0670220c8fbe183d07"
		if r.Header.Get(aidataplane.TargetHeader) != target || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("model pipeline target/path = %q/%q, want selected target and Chat Completions", r.Header.Get(aidataplane.TargetHeader), r.URL.Path)
		}

		if r.Header.Get(aidataplane.ListenerPortHeader) != "" || r.Header.Get(aidataplane.HostNameHeader) != "" {
			t.Errorf("model pipeline forwarded entry headers to egress: %v", r.Header)
		}

		w.WriteHeader(http.StatusAccepted)
		if _, err := io.WriteString(w, `{"accepted":true}`); err != nil {
			t.Errorf("WriteString(model response) error = %v", err)
		}
	})
	peer := &httptest.Server{Listener: egress, Config: &http.Server{Handler: peerHandler}}
	peer.Start()
	t.Cleanup(peer.Close)
	ingress := unixListener(t, "ingress.sock")

	synctest.Test(t, func(t *testing.T) {
		loader := loadedConfig(t)
		httpServer, closeIdle, err := NewHTTPServer(ingress, loader, egress.Addr().String(), 1024, slog.Default())
		if err != nil {
			t.Fatalf("NewHTTPServer(valid dependencies) error = %v", err)
		}

		t.Cleanup(closeIdle)
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"coding","messages":[]}`))
		request.Header.Set(aidataplane.ListenerPortHeader, "8080")
		request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")
		response := httptest.NewRecorder()

		httpServer.ServeHTTP(response, request)

		if response.Code != http.StatusAccepted || response.Body.String() != `{"accepted":true}` || calls.Load() != 1 {
			t.Errorf("assembled model pipeline = (%d, %s, calls %d), want unchanged peer response and one call", response.Code, response.Body, calls.Load())
		}

		endpoint, err := httpServer.Endpoint()
		if err != nil || endpoint.Scheme != "unix" || endpoint.Path != ingress.Addr().String() {
			t.Errorf("Endpoint(assembled server) = (%v, %v), want supplied Unix listener", endpoint, err)
		}

		for _, tt := range []struct {
			name       string
			path       string
			body       string
			trusted    bool
			hostName   string
			wantStatus int
			wantCode   string
		}{
			{name: "missing entry", path: "/v1/chat/completions", body: `{"model":"coding"}`, wantStatus: http.StatusBadRequest, wantCode: "INVALID_MODEL_ENTRY"},
			{name: "unknown model", path: "/v1/chat/completions", body: `{"model":"other"}`, trusted: true, wantStatus: http.StatusNotFound, wantCode: "MODEL_NOT_FOUND"},
			{name: "other namespace", path: "/v1/chat/completions", body: `{"model":"coding"}`, trusted: true, hostName: "gateway/team-b/public/listener/api", wantStatus: http.StatusNotFound, wantCode: "MODEL_NOT_FOUND"},
			{name: "oversized body", path: "/v1/chat/completions", body: strings.Repeat("x", 1025), trusted: true, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "MODEL_REQUEST_TOO_LARGE"},
			{name: "invalid JSON", path: "/v1/chat/completions", body: `{"model":`, trusted: true, wantStatus: http.StatusBadRequest, wantCode: "MODEL_INVALID_ARGUMENT"},
			{name: "unimplemented operation", path: "/v1/responses/resp_1", body: `{}`, trusted: true, wantStatus: http.StatusNotFound},
			// 路径与全局诊断 mux 的注册项相同，仍须由 MCP 入口拒绝而不是回退到诊断 handler。
			{name: "global diagnostic mux", path: "/unintended-debug/", body: `{}`, trusted: true, wantStatus: http.StatusNotFound},
		} {
			request := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			request.Header.Set("x-ai-eg-model", "coding")
			request.Header.Set(aidataplane.TargetHeader, "spoofed")
			if tt.trusted {
				request.Header.Set(aidataplane.ListenerPortHeader, "8080")
				request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")
			}

			if tt.hostName != "" {
				request.Header.Set(aidataplane.HostNameHeader, tt.hostName)
			}

			response := httptest.NewRecorder()

			httpServer.ServeHTTP(response, request)

			if response.Code != tt.wantStatus || calls.Load() != 1 {
				t.Errorf("assembled model pipeline(%s) status/calls = %d/%d, want %d/1 without another peer call", tt.name, response.Code, calls.Load(), tt.wantStatus)
			}

			if tt.wantCode != "" {
				var result openai.ErrorResponse
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatalf("decode local model error(%s): %v", tt.name, err)
				}

				if result.Error.Code == nil || *result.Error.Code != tt.wantCode || result.Error.Type != "invalid_request_error" || result.Error.Message == "" {
					t.Errorf("local model error(%s) = %+v, want invalid_request_error/%s", tt.name, result.Error, tt.wantCode)
				}
			}
		}
	})
}

func TestNewHTTPServerLocalUnavailableErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		loaded bool
		status int
		code   string
	}{
		{name: "configuration not loaded", status: 503, code: "MODEL_UNAVAILABLE"},
		{name: "egress socket unavailable", loaded: true, status: 502, code: "MODEL_FORWARD_FAILED"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				loader := aiconfig.NewLoader(fake.NewClientset(), "control-plane", logger)
				if tt.loaded {
					loader = loadedConfig(t)
				}

				listener := unixListener(t, "ingress.sock")
				egress := filepath.Join(unixSocketDirectory(t), "private-egress.sock")
				httpServer, closeIdle, err := NewHTTPServer(listener, loader, egress, 1024, logger)
				if err != nil {
					t.Fatalf("create model server: %v", err)
				}

				t.Cleanup(closeIdle)
				request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"coding"}`))
				request.Header.Set(aidataplane.ListenerPortHeader, "8080")
				request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")
				response := httptest.NewRecorder()

				httpServer.ServeHTTP(response, request)

				var result openai.ErrorResponse
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatalf("decode unavailable error: %v", err)
				}

				if response.Code != tt.status || result.Error.Code == nil || *result.Error.Code != tt.code || result.Error.Type != "server_error" {
					t.Errorf("unavailable error = %d/%+v, want %d/server_error/%s", response.Code, result.Error, tt.status, tt.code)
				}

				if strings.Contains(response.Body.String(), "private-egress") || strings.Contains(response.Body.String(), "dial unix") {
					t.Errorf("unavailable error exposed socket details: %s", response.Body)
				}
			})
		})
	}
}

func TestNewHTTPServerRejectsInvalidConfiguration(t *testing.T) {
	listener := unixListener(t, "ingress.sock")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := aiconfig.NewLoader(fake.NewClientset(), "control-plane", logger)

	for _, tt := range []struct {
		name   string
		socket string
		limit  int64
	}{
		{name: "relative egress", socket: "relative.sock", limit: 1},
		{name: "zero body limit", socket: "/unused/egress.sock"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			httpServer, closeIdle, err := NewHTTPServer(listener, loader, tt.socket, tt.limit, logger)

			if httpServer != nil || closeIdle != nil || err == nil {
				t.Errorf("NewHTTPServer(%s) = (%v, cleanup %t, %v), want nil results and error", tt.name, httpServer, closeIdle != nil, err)
			}

			raw, err := listener.SyscallConn()
			if err != nil {
				t.Fatalf("SyscallConn(caller's listener) error = %v", err)
			}

			if err := raw.Control(func(uintptr) {}); err != nil {
				t.Errorf("NewHTTPServer(%s) closed caller's listener: %v", tt.name, err)
			}
		})
	}
}

func TestModelPipelineStreamsBeforeCompletionAndCancelsUpstream(t *testing.T) {
	for _, tt := range []struct {
		name  string
		path  string
		event string
	}{
		{name: "Chat Completions", path: "/v1/chat/completions", event: "data: first\n\n"},
		{name: "Responses", path: "/v1/responses", event: "event: response.created\ndata: {\"type\":\"response.created\"}\n\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			testModelPipelineStream(t, tt.path, tt.event)
		})
	}
}

// testModelPipelineStream 在对端保持连接时读取完整首帧，再检查取消能到达出口。
func testModelPipelineStream(t *testing.T, operationPath, event string) {
	t.Helper()
	deadline, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	ctx, cancel := context.WithCancel(deadline)
	defer cancel()
	ended := make(chan struct{})
	egress := unixListener(t, "egress.sock")
	peerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(ended)
		if r.URL.Path != operationPath {
			t.Errorf("model stream path = %q, want %q", r.URL.Path, operationPath)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := io.WriteString(w, event); err != nil {
			t.Errorf("WriteString(first event) error = %v", err)
			return
		}

		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush(first event) error = %v", err)
			return
		}

		<-r.Context().Done()
	})
	peer := &httptest.Server{Listener: egress, Config: &http.Server{Handler: peerHandler}}
	peer.Start()
	t.Cleanup(peer.Close)
	ingress := unixListener(t, "ingress.sock")

	synctest.Test(t, func(t *testing.T) {
		httpServer, closeIdle, err := NewHTTPServer(ingress, loadedConfig(t), egress.Addr().String(), 1024, slog.Default())
		if err != nil {
			t.Fatalf("NewHTTPServer(stream) error = %v", err)
		}

		t.Cleanup(closeIdle)
		entry := &httptest.Server{Listener: ingress, Config: &http.Server{Handler: httpServer}}
		entry.Start()
		t.Cleanup(entry.Close)
		client, err := envoyclient.New(ingress.Addr().String())
		if err != nil {
			t.Fatalf("New(Unix ingress client) error = %v", err)
		}

		t.Cleanup(client.CloseIdleConnections)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://ingate-ai-ingress"+operationPath, strings.NewReader(`{"model":"coding","stream":true}`))
		if err != nil {
			t.Fatalf("NewRequestWithContext(stream) error = %v", err)
		}

		request.Header.Set(aidataplane.ListenerPortHeader, "8080")
		request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")

		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("Do(stream) error = %v", err)
		}

		defer func() {
			if err := response.Body.Close(); err != nil {
				t.Errorf("Close(stream response) error = %v", err)
			}
		}()
		first := make([]byte, len(event))
		_, err = io.ReadFull(response.Body, first)

		if err != nil || string(first) != event {
			t.Fatalf("ReadFull(%s first event) = (%q, %v), want %q while upstream is still open", operationPath, first, err, event)
		}

		cancel()

		select {
		case <-ended:
		case <-deadline.Done():
			t.Error("client cancellation did not reach the Unix peer through Handler and Service")
		}
	})
}

// loadedConfig 只在 synctest 中运行；假 API 只替换远端服务，保留真实 Loader。
func loadedConfig(t *testing.T) *aiconfig.Loader {
	t.Helper()
	data, err := os.ReadFile("../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(compiled configuration) error = %v", err)
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "control-plane", Name: aidataplane.ConfigMapName, UID: "fixture",
		},
		Data: map[string]string{aidataplane.ConfigMapDataKey: string(data)},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := aiconfig.NewLoader(fake.NewClientset(configMap), "control-plane", logger)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- loader.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run(stopped Loader) error = %v, want context.Canceled", err)
		}
	})
	synctest.Wait()
	if _, loaded := loader.Capture(); !loaded {
		t.Fatal("Capture(model pipeline) has no configuration, want loaded fixture")
	}

	return loader
}

func unixListener(t *testing.T, name string) *net.UnixListener {
	t.Helper()
	directory := unixSocketDirectory(t)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(directory, name), Net: "unix"})
	if err != nil {
		t.Fatalf("ListenUnix(test listener) error = %v", err)
	}

	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("Close(test listener) error = %v", err)
		}
	})

	return listener
}

func unixSocketDirectory(t *testing.T) string {
	t.Helper()
	// MkdirTemp 明确创建私有目录；短前缀避免 macOS 的 Unix 路径长度限制。
	directory, err := os.MkdirTemp("", "ig-app-")
	if err != nil {
		t.Fatalf("MkdirTemp(Unix listener) error = %v", err)
	}

	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Errorf("RemoveAll(test socket directory) error = %v", err)
		}
	})

	return directory
}
