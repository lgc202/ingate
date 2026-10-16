package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/service/mcpproxy"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestMCPProxyHandlerForwards(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		called := false
		transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("ReadAll(outbound body) error = %v", err)
			}

			if r.Method != http.MethodPost || r.URL.EscapedPath() != "/stream" || r.URL.RawQuery != "" {
				t.Errorf("Forward() outbound = %s %s, want the configured backend path", r.Method, r.URL)
			}

			if r.Header.Get(aidataplane.TargetHeader) == "" || r.Header.Get(aidataplane.TargetHeader) == "spoofed" {
				t.Errorf("Forward() target = %q, want generated target", r.Header.Get(aidataplane.TargetHeader))
			}

			if r.Header.Get("Mcp-Protocol-Version") != "2026-07-28" || r.Header.Get("Origin") != "http://example.com" {
				t.Errorf("Forward() protocol/Origin headers = %v, want forwarded MCP metadata", r.Header)
			}

			if r.Header.Get("Authorization") != "" || r.Header.Get(aidataplane.ListenerPortHeader) != "" {
				t.Errorf("Forward() retained private header: %v", r.Header)
			}

			return modelResponse(http.StatusOK, string(body), http.Header{"Content-Type": {"application/json"}}), nil
		})
		service := startMCPService(t, transport, modelConfigMap(t))
		router := newMCPRouter(t, service, 1024)
		request := trustedMCPRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`)
		request.Header.Set("Mcp-Protocol-Version", "2026-07-28")
		request.Header.Set("Origin", "http://example.com")
		request.Header.Set("Authorization", "private credential")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if !called || response.Code != http.StatusOK || response.Body.String() != `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}` {
			t.Errorf("Forward() = (called %t, status %d, body %s), want original upstream response", called, response.Code, response.Body)
		}
	})
}

func TestMCPProxyHandlerRejectsNonPostMethods(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var method string
		transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			method = r.Method
			return modelResponse(http.StatusNoContent, "", nil), nil
		})
		service := startMCPService(t, transport, modelConfigMap(t))
		router := newMCPRouter(t, service, 1024)
		response := httptest.NewRecorder()

		router.ServeHTTP(response, trustedMCPRequest(t, http.MethodDelete, "/mcp", ""))

		if method != "" || response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "POST" {
			t.Errorf("Forward(DELETE) = (%s, %d), want 405 with Allow and no upstream call", method, response.Code)
		}
	})
}

func TestMCPProxyHandlerRejectsOrigin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		called := false
		transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return modelResponse(http.StatusNoContent, "", nil), nil
		})
		service := startMCPService(t, transport, modelConfigMap(t))
		router := newMCPRouter(t, service, 1024)

		for _, tt := range []struct {
			name   string
			origin string
			want   int
		}{
			{name: "cross origin", origin: "http://evil.example", want: http.StatusForbidden},
			{name: "opaque origin", origin: "null", want: http.StatusForbidden},
			{name: "same origin", origin: "http://example.com", want: http.StatusNoContent},
			{name: "absent origin", origin: "", want: http.StatusNoContent},
		} {
			called = false
			response := httptest.NewRecorder()
			request := trustedMCPRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`)
			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}

			router.ServeHTTP(response, request)

			if response.Code != tt.want || called != (tt.want == http.StatusNoContent) {
				t.Errorf("Forward(origin %q) = (status %d, called %t), want %d/%t", tt.name, response.Code, called, tt.want, tt.want == http.StatusNoContent)
			}
		}
	})
}

func TestMCPProxyHandlerRejectsBeforeClientCall(t *testing.T) {
	for _, tt := range []struct {
		name       string
		method     string
		path       string
		body       string
		limit      int64
		trusted    bool
		wantStatus int
	}{
		{name: "missing entry", method: http.MethodPost, path: "/mcp", body: `{}`, limit: 1024, wantStatus: http.StatusServiceUnavailable},
		{name: "unknown path", method: http.MethodPost, path: "/mcpx", body: `{}`, limit: 1024, trusted: true, wantStatus: http.StatusNotFound},
		{name: "trailing slash", method: http.MethodPost, path: "/mcp/", body: `{}`, limit: 1024, trusted: true, wantStatus: http.StatusNotFound},
		{name: "over limit", method: http.MethodPost, path: "/mcp", body: `{"jsonrpc":"2.0"}`, limit: 1, trusted: true, wantStatus: http.StatusRequestEntityTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				called := false
				transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return modelResponse(http.StatusNoContent, "", nil), nil
				})
				service := startMCPService(t, transport, modelConfigMap(t))
				router := newMCPRouter(t, service, tt.limit)
				request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				if tt.trusted {
					request = request.WithContext(WithEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
				}

				response := httptest.NewRecorder()

				router.ServeHTTP(response, request)

				if response.Code != tt.wantStatus || called {
					t.Errorf("Forward(%s) status/called = %d/%t, want %d/false", tt.name, response.Code, called, tt.wantStatus)
				}
			})
		})
	}
}

func TestMCPProxyHandlerMiddlewarePrecedesRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		called := false
		transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return modelResponse(http.StatusNoContent, "", nil), nil
		})
		service := startMCPService(t, transport, modelConfigMap(t))
		deny := func(middleware.Handler) middleware.Handler {
			return func(context.Context, any) (any, error) {
				return nil, kratoserrors.Forbidden("TEST_DENIED", "access denied")
			}
		}
		router := newMCPRouter(t, service, 1024, kratoshttp.Middleware(deny))
		body := &failingBody{err: errors.New("must not read unauthorized body")}
		request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		request.Body = body
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusForbidden || called || body.reads != 0 {
			t.Errorf("Forward(denied) status/called/reads = %d/%t/%d, want 403/false/0", response.Code, called, body.reads)
		}
	})
}

func TestNewMCPProxyRejectsInvalidBodyLimit(t *testing.T) {
	loader := aiconfig.NewLoader(fake.NewClientset(), "control-plane", slog.Default())

	service := mcpproxy.New(loader, &http.Client{})
	for _, limit := range []int64{0, -1} {
		mcpProxyHandler, err := NewMCPProxy(service, limit, slog.Default())

		if mcpProxyHandler != nil || err == nil {
			t.Errorf("NewMCPProxy(limit %d) = (%v, %v), want nil and configuration error", limit, mcpProxyHandler, err)
		}
	}
}

func TestMCPTransportError(t *testing.T) {
	for _, tt := range []struct {
		name        string
		cause       error
		wantStatus  int
		wantReason  string
		wantMessage string
	}{
		{
			name:        "route not found",
			cause:       mcpproxy.ErrRouteNotFound,
			wantStatus:  404,
			wantReason:  "MCP_ROUTE_NOT_FOUND",
			wantMessage: "MCP route is not available at this entry",
		},
		{
			name:        "unavailable",
			cause:       mcpproxy.ErrUnavailable,
			wantStatus:  503,
			wantReason:  "MCP_UNAVAILABLE",
			wantMessage: "MCP service is unavailable",
		},
		{
			name:        "forward failed",
			cause:       mcpproxy.ErrForwardFailed,
			wantStatus:  502,
			wantReason:  "MCP_FORWARD_FAILED",
			wantMessage: "MCP forwarding failed",
		},
		{
			name:        "canceled",
			cause:       fmt.Errorf("%w: %w", mcpproxy.ErrForwardFailed, context.Canceled),
			wantStatus:  499,
			wantReason:  "REQUEST_CANCELED",
			wantMessage: "request was canceled",
		},
		{
			name:        "deadline",
			cause:       fmt.Errorf("%w: %w", mcpproxy.ErrForwardFailed, context.DeadlineExceeded),
			wantStatus:  504,
			wantReason:  "DEADLINE_EXCEEDED",
			wantMessage: "request deadline exceeded",
		},
		{
			name:        "unknown error",
			cause:       errors.New("private failure"),
			wantStatus:  500,
			wantReason:  "INTERNAL_SERVER_ERROR",
			wantMessage: "internal server error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			mapped := toMCPTransportError(t.Context(), fmt.Errorf("private cause: %w", tt.cause), logger)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/mcp", nil)

			kratoshttp.DefaultErrorEncoder(response, request, mapped)

			if response.Code != tt.wantStatus || !errors.Is(mapped, tt.cause) {
				t.Errorf("toMCPTransportError(%s) = (%d, %v), want %d with cause", tt.name, response.Code, mapped, tt.wantStatus)
			}

			var result struct {
				Code    int    `json:"code"`
				Reason  string `json:"reason"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode toMCPTransportError(%s) response: %v", tt.name, err)
			}

			if result.Code != tt.wantStatus || result.Reason != tt.wantReason || result.Message != tt.wantMessage {
				t.Errorf("toMCPTransportError(%s) response = %+v, want %d/%s/%s", tt.name, result, tt.wantStatus, tt.wantReason, tt.wantMessage)
			}

			if strings.Contains(response.Body.String(), "private") {
				t.Errorf("toMCPTransportError(%s) exposed private details in %s", tt.name, response.Body)
			}
		})
	}
}

func TestMCPProxyHandlerRejectsMetadata(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startMCPService(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Error("invalid metadata reached upstream")
			return modelResponse(200, "", nil), nil
		}), modelConfigMap(t))
		router := newMCPRouter(t, service, 1024)
		const body = `{"jsonrpc":"2.0","id":9007199254740993,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
		for _, tt := range []struct {
			name   string
			change func(http.Header)
			code   int
		}{
			{name: "missing version", change: func(h http.Header) { h.Del("MCP-Protocol-Version") }, code: -32020},
			{name: "duplicate version", change: func(h http.Header) { h.Add("MCP-Protocol-Version", "2026-07-28") }, code: -32020},
			{name: "unsupported version", change: func(h http.Header) { h.Set("MCP-Protocol-Version", "2025-11-25") }, code: -32022},
			{name: "method mismatch", change: func(h http.Header) { h.Set("Mcp-Method", "tools/call") }, code: -32020},
		} {
			request := trustedMCPRequest(t, http.MethodPost, "/mcp", body)
			tt.change(request.Header)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			var result struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Error   struct {
					Code int                        `json:"code"`
					Data map[string]json.RawMessage `json:"data"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}

			if response.Code != 400 || result.JSONRPC != "2.0" || string(result.ID) != "9007199254740993" || result.Error.Code != tt.code {
				t.Errorf("%s: status=%d body=%s", tt.name, response.Code, response.Body)
			}

			if tt.code == -32022 && (string(result.Error.Data["requested"]) != `"2025-11-25"` || string(result.Error.Data["supported"]) != `["2026-07-28"]`) {
				t.Errorf("unsupported version data=%v", result.Error.Data)
			}
		}
	})
}

// newMCPRouter 注册与 server.New 相同的 MCP 通配路由，用于直接测试 Handler 行为。
func newMCPRouter(
	t *testing.T,
	service *mcpproxy.Service,
	limit int64,
	options ...kratoshttp.ServerOption,
) *kratoshttp.Server {
	t.Helper()
	mcpProxyHandler, err := NewMCPProxy(service, limit, slog.Default())
	if err != nil {
		t.Fatalf("NewMCPProxy(test router) error = %v", err)
	}

	options = append([]kratoshttp.ServerOption{kratoshttp.Timeout(0), kratoshttp.StrictSlash(false)}, options...)
	router := kratoshttp.NewServer(options...)
	router.Route("/").Handle("*", "/{path:.*}", mcpProxyHandler.Forward)

	return router
}

// startMCPService 使用真实 Loader 与原生 Transport 测试替身构造完整 MCP 用例。
func startMCPService(t *testing.T, transport http.RoundTripper, objects ...runtime.Object) *mcpproxy.Service {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	apiClient := fake.NewClientset(objects...)
	loader := aiconfig.NewLoader(apiClient, "control-plane", logger)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- loader.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run(stopped loader) error = %v, want context.Canceled", err)
		}
	})
	synctest.Wait()

	return mcpproxy.New(loader, &http.Client{Transport: transport})
}

func trustedMCPRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("MCP-Protocol-Version", "2026-07-28")
	request.Header.Set("Mcp-Method", "tools/list")

	return request.WithContext(WithEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
}
