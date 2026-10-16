package aiproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/envoyclient"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestNewHTTPServerConnectsMCPPipeline(t *testing.T) {
	var calls atomic.Int32
	egress := unixListener(t, "egress.sock")
	peerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll(MCP pipeline) error = %v", err)
			return
		}

		if r.URL.Path != "/stream" || r.URL.RawQuery != "" {
			t.Errorf("MCP pipeline path = %q, want the configured backend path", r.URL)
		}

		const target = "ai/mcp-target/f302e6faf73db6eca1dab4385f17012e06b05d90e0ca416ddfcec9aa73fb4af3"
		if r.Header.Get(aidataplane.TargetHeader) != target {
			t.Errorf("MCP pipeline target = %q, want the tools network target", r.Header.Get(aidataplane.TargetHeader))
		}

		if r.Header.Get(aidataplane.ListenerPortHeader) != "" || r.Header.Get(aidataplane.HostNameHeader) != "" {
			t.Errorf("MCP pipeline forwarded entry headers to egress: %v", r.Header)
		}

		if r.Header.Get("Mcp-Protocol-Version") != "2026-07-28" {
			t.Errorf("MCP pipeline protocol header = %q, want forwarded metadata", r.Header.Get("Mcp-Protocol-Version"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(body); err != nil {
			t.Errorf("Write(MCP response) error = %v", err)
		}
	})
	peer := &httptest.Server{Listener: egress, Config: &http.Server{Handler: peerHandler}}
	peer.Start()
	t.Cleanup(peer.Close)
	ingress := unixListener(t, "ingress.sock")

	synctest.Test(t, func(t *testing.T) {
		httpServer, closeIdle, err := NewHTTPServer(ingress, loadedConfig(t), egress.Addr().String(), 1024, slog.Default())
		if err != nil {
			t.Fatalf("NewHTTPServer(valid dependencies) error = %v", err)
		}

		t.Cleanup(closeIdle)
		request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`))
		request.Header.Set(aidataplane.ListenerPortHeader, "8080")
		request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")
		request.Header.Set("Mcp-Protocol-Version", "2026-07-28")
		request.Header.Set("Mcp-Method", "tools/list")
		response := httptest.NewRecorder()

		httpServer.ServeHTTP(response, request)

		if response.Code != http.StatusOK || response.Body.String() != `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}` || calls.Load() != 1 {
			t.Errorf("assembled MCP pipeline = (%d, %s, calls %d), want unchanged peer response and one call", response.Code, response.Body, calls.Load())
		}

		deleteRequest := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
		deleteRequest.Header.Set(aidataplane.ListenerPortHeader, "8080")
		deleteRequest.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")
		deleteRequest.Header.Set(aidataplane.EntryProtocolHeader, "mcp")
		deleteResponse := httptest.NewRecorder()

		httpServer.ServeHTTP(deleteResponse, deleteRequest)

		if deleteResponse.Code != http.StatusMethodNotAllowed || calls.Load() != 1 || deleteResponse.Header().Get("Allow") != "POST" {
			t.Errorf("assembled MCP pipeline(DELETE) = (%d, calls %d), want 405 and no upstream call", deleteResponse.Code, calls.Load())
		}

		for _, tt := range []struct {
			name       string
			method     string
			path       string
			entry      bool
			origin     string
			wantStatus int
			wantCode   string
		}{
			{name: "missing entry", method: http.MethodPost, path: "/mcp", wantStatus: http.StatusBadRequest, wantCode: "INVALID_MCP_ENTRY"},
			{name: "unknown path", method: http.MethodPost, path: "/unknown", entry: true, wantStatus: http.StatusNotFound, wantCode: "MCP_ROUTE_NOT_FOUND"},
			{name: "trailing slash is a different entry", method: http.MethodPost, path: "/mcp/", entry: true, wantStatus: http.StatusNotFound, wantCode: "MCP_ROUTE_NOT_FOUND"},
			{name: "cross origin", method: http.MethodPost, path: "/mcp", entry: true, origin: "http://evil.example", wantStatus: http.StatusForbidden, wantCode: "MCP_ORIGIN_REJECTED"},
			{name: "non-post model path rejected by MCP entry", method: http.MethodGet, path: "/v1/chat/completions", entry: true, wantStatus: http.StatusNotFound, wantCode: "MCP_ROUTE_NOT_FOUND"},
		} {
			before := calls.Load()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			if tt.entry {
				request.Header.Set(aidataplane.ListenerPortHeader, "8080")
				request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")
			}

			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}

			response := httptest.NewRecorder()

			httpServer.ServeHTTP(response, request)

			if response.Code != tt.wantStatus || calls.Load() != before {
				t.Errorf("assembled MCP pipeline(%s) status/calls = %d/%d, want %d without another peer call", tt.name, response.Code, calls.Load(), tt.wantStatus)
			}

			var result struct {
				JSONRPC string `json:"jsonrpc"`
				Error   struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode local MCP error(%s): %v", tt.name, err)
			}

			if result.JSONRPC != "2.0" || result.Error.Code != -32600 || result.Error.Message == "" {
				t.Errorf("local MCP error(%s) = %+v, want code %s", tt.name, result.Error, tt.wantCode)
			}
		}
	})
}

// TestMCPPipelineStreamsBeforeCompletionAndCancelsUpstream 在对端保持连接时读取完整首帧，
// 再确认客户端取消能沿 MCP Handler 与 Service 到达 Unix 出口。
func TestMCPPipelineStreamsBeforeCompletionAndCancelsUpstream(t *testing.T) {
	deadline, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	ctx, cancel := context.WithCancel(deadline)
	defer cancel()
	ended := make(chan struct{})
	egress := unixListener(t, "egress.sock")
	peerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(ended)
		if r.URL.Path != "/stream" {
			t.Errorf("MCP stream path = %q, want the configured backend path", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := io.WriteString(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{\"level\":\"info\",\"data\":\"started\"}}\n\n"); err != nil {
			t.Errorf("WriteString(first MCP event) error = %v", err)
			return
		}

		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush(first MCP event) error = %v", err)
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
			t.Fatalf("NewHTTPServer(MCP stream) error = %v", err)
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
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://ingate-ai-ingress/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`))
		if err != nil {
			t.Fatalf("NewRequestWithContext(MCP stream) error = %v", err)
		}

		request.Header.Set(aidataplane.ListenerPortHeader, "8080")
		request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")
		request.Header.Set("Mcp-Protocol-Version", "2026-07-28")
		request.Header.Set("Mcp-Method", "tools/list")
		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("Do(MCP stream) error = %v", err)
		}

		defer func() {
			if err := response.Body.Close(); err != nil {
				t.Errorf("Close(MCP stream response) error = %v", err)
			}
		}()
		const event = "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{\"level\":\"info\",\"data\":\"started\"}}\n\n"
		first := make([]byte, len(event))
		_, err = io.ReadFull(response.Body, first)

		if err != nil || string(first) != event {
			t.Fatalf("ReadFull(MCP first event) = (%q, %v), want %q while upstream is still open", first, err, event)
		}

		cancel()

		select {
		case <-ended:
		case <-deadline.Done():
			t.Error("client cancellation did not reach the Unix peer through MCP Handler and Service")
		}
	})
}

// TestMCPPipelineExactEntriesAndResponses 使用双 UDS 验证协议标记只选择入口类型，
// 路径和目标仍须来自本次捕获配置；不允许透传客户端选择或回退到模型路径。
func TestMCPPipelineExactEntriesAndResponses(t *testing.T) {
	var calls atomic.Int32
	egress := unixListener(t, "egress.sock")
	peer := &httptest.Server{Listener: egress, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}

		if r.URL.Path != "/stream/" || r.URL.RawQuery != "" {
			t.Errorf("outbound path=%s", r.URL)
		}

		if r.Header.Get(aidataplane.EntryProtocolHeader) != "" || r.Header.Get("X-Envoy-Max-Retries") != "" {
			t.Error("internal/control headers reached egress")
		}

		if r.Header.Get("Mcp-Param-Unrecognized") != "=?base64?opaque?=" {
			t.Error("unknown MCP parameter header lost")
		}

		if strings.Contains(string(body), `"method":"custom/notification"`) {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		if string(body) != `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}` {
			t.Errorf("modified JSON request bytes: %s", body)
		}

		if r.Header.Get("X-Test-Response") == "error" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			_, err = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"busy"}}`)
		} else {
			w.Header().Set("Content-Type", "application/json")
			_, err = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`)
		}

		if err != nil {
			t.Error(err)
		}
	})}}
	peer.Start()
	t.Cleanup(peer.Close)
	ingress := unixListener(t, "ingress.sock")

	synctest.Test(t, func(t *testing.T) {
		data, err := os.ReadFile("../pkg/aidataplane/testdata/v1.json")
		if err != nil {
			t.Fatal(err)
		}

		compiled, err := aidataplane.Decode(data)
		if err != nil {
			t.Fatal(err)
		}

		host := &compiled.Listeners[0].Hosts[0]
		host.ModelRoutes = []aidataplane.ModelRoute{}
		route := host.MCPRoutes[0]
		route.BackendPath = "/stream/"
		host.MCPRoutes = []aidataplane.MCPRoute{}
		for i, path := range []string{"/mcp/", aidataplane.ModelResponsesPath, aidataplane.ModelChatCompletionsPath} {
			candidate := route
			candidate.Name = fmt.Sprintf("team-a/tools-%d", i)
			candidate.Path = path
			host.MCPRoutes = append(host.MCPRoutes, candidate)
		}

		payload, err := aidataplane.Encode(*compiled)
		if err != nil {
			t.Fatal(err)
		}

		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "control-plane", Name: aidataplane.ConfigMapName, UID: "fixture"}, Data: map[string]string{aidataplane.ConfigMapDataKey: string(payload)}}
		apiClient := fake.NewClientset(configMap)
		loader := aiconfig.NewLoader(apiClient, "control-plane", slog.Default())
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- loader.Run(ctx) }()
		t.Cleanup(func() {
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Error(err)
			}
		})
		synctest.Wait()

		server, closeIdle, err := NewHTTPServer(ingress, loader, egress.Addr().String(), 1024, slog.Default())
		if err != nil {
			t.Fatal(err)
		}

		t.Cleanup(closeIdle)
		entry := &httptest.Server{Listener: ingress, Config: &http.Server{Handler: server}}
		entry.Start()
		t.Cleanup(entry.Close)
		client, err := envoyclient.New(ingress.Addr().String())
		if err != nil {
			t.Fatal(err)
		}

		t.Cleanup(client.CloseIdleConnections)

		const body = `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
		for _, tt := range []struct {
			name, method, path, body, reply, host string
			port                                  string
			status                                int
			forward                               bool
		}{
			{name: "trailing slash", method: "POST", path: "/mcp/?ignored=yes", body: body, status: 200, forward: true},
			{name: "MCP-only responses path", method: "POST", path: aidataplane.ModelResponsesPath, body: body, status: 200, forward: true},
			{name: "MCP-only chat path", method: "POST", path: aidataplane.ModelChatCompletionsPath, body: body, status: 200, forward: true},
			{name: "notification", method: "POST", path: "/mcp/", body: `{"jsonrpc":"2.0","method":"custom/notification"}`, status: 202, forward: true},
			{name: "upstream rejection", method: "POST", path: "/mcp/", body: body, reply: "error", status: 429, forward: true},
			{name: "GET", method: "GET", path: "/mcp/", status: 405},
			{name: "HEAD", method: "HEAD", path: "/mcp/", status: 405},
			{name: "OPTIONS", method: "OPTIONS", path: "/mcp/", status: 405},
			{name: "DELETE", method: "DELETE", path: aidataplane.ModelResponsesPath, status: 405},
			{name: "slash mismatch", method: "POST", path: "/mcp", body: body, status: 404},
			{name: "other namespace", method: "POST", path: "/mcp/", body: body, host: "gateway/team-b/public/listener/api", status: 404},
			{name: "other port", method: "POST", path: "/mcp/", body: body, port: "8081", status: 404},
		} {
			before := calls.Load()
			request, err := http.NewRequestWithContext(t.Context(), tt.method, "http://example.com"+tt.path, strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}

			port := tt.port
			if port == "" {
				port = "8080"
			}

			hostName := tt.host
			if hostName == "" {
				hostName = "gateway/team-a/public/listener/api"
			}

			request.Header.Set(aidataplane.ListenerPortHeader, port)
			request.Header.Set(aidataplane.HostNameHeader, hostName)
			request.Header.Set(aidataplane.EntryProtocolHeader, "mcp")
			request.Header.Set("MCP-Protocol-Version", "2026-07-28")
			request.Header.Set("Mcp-Method", "tools/list")
			request.Header.Set("Mcp-Param-Unrecognized", "=?base64?opaque?=")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			request.Header.Set("Origin", "http://example.com:80")
			request.Header.Set("X-Test-Response", tt.reply)
			request.Header.Set("X-Envoy-Max-Retries", "4")

			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}

			received, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}

			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}

			if response.StatusCode != tt.status || (calls.Load() > before) != tt.forward {
				t.Fatalf("%s status/calls=%d/%d, body=%s", tt.name, response.StatusCode, calls.Load()-before, received)
			}

			if tt.status == 405 && response.Header.Get("Allow") != "POST" {
				t.Errorf("%s missing Allow: POST", tt.name)
			}

			if tt.status == 200 && string(received) != `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}` {
				t.Errorf("%s modified JSON response bytes", tt.name)
			}

			if tt.status == 202 && len(received) != 0 {
				t.Error("notification has response body")
			}

			if tt.status == 429 && (response.Header.Get("Retry-After") != "2" || !strings.Contains(string(received), `"message":"busy"`)) {
				t.Error("upstream rejection changed")
			}
		}
	})
}
