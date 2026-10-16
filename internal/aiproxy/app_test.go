package aiproxy

import (
	"bufio"
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
		}{
			{name: "missing entry", path: "/v1/chat/completions", body: `{"model":"coding"}`, wantStatus: http.StatusBadRequest},
			{name: "unknown model", path: "/v1/chat/completions", body: `{"model":"other"}`, trusted: true, wantStatus: http.StatusNotFound},
			{name: "other namespace", path: "/v1/chat/completions", body: `{"model":"coding"}`, trusted: true, hostName: "gateway/team-b/public/listener/api", wantStatus: http.StatusNotFound},
			{name: "oversized body", path: "/v1/chat/completions", body: strings.Repeat("x", 1025), trusted: true, wantStatus: http.StatusRequestEntityTooLarge},
			{name: "unimplemented operation", path: "/v1/responses", body: `{}`, trusted: true, wantStatus: http.StatusNotFound},
			{name: "global diagnostic mux", path: "/unintended-debug/", body: `{}`, wantStatus: http.StatusNotFound},
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
		}
	})
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
	deadline, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	ctx, cancel := context.WithCancel(deadline)
	defer cancel()
	ended := make(chan struct{})
	egress := unixListener(t, "egress.sock")
	peerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(ended)
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := io.WriteString(w, "data: first\n\n"); err != nil {
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
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://ingate-ai-ingress/v1/chat/completions", strings.NewReader(`{"model":"coding","stream":true}`))
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
		first, err := bufio.NewReader(response.Body).ReadString('\n')

		if err != nil || first != "data: first\n" {
			t.Fatalf("ReadString(first event) = (%q, %v), want event while upstream is still open", first, err)
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
