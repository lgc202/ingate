package aiproxy

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/google/go-cmp/cmp"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/apischema/openai"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestResponsesPipelinePreservesUpstreamMessages(t *testing.T) {
	for _, tt := range []struct {
		name        string
		status      int
		contentType string
		body        string
	}{
		{name: "JSON", status: 200, contentType: "application/json", body: `{"id":"resp_1","model":"code-v2","output":[]}`},
		{name: "SSE", status: 200, contentType: "text/event-stream", body: "event: response.created\ndata: {\"type\":\"response.created\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\"}\n\n"},
		{name: "provider error", status: 429, contentType: "application/json", body: `{"error":{"message":"rate limited","type":"rate_limit_error"}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			egress := unixListener(t, "egress.sock")
			peerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("ReadAll(Responses upstream request) error = %v", err)
					return
				}

				var fields map[string]json.RawMessage
				if err := json.Unmarshal(body, &fields); err != nil {
					t.Errorf("Unmarshal(Responses upstream request) error = %v", err)
					return
				}

				want := map[string]json.RawMessage{
					"model": json.RawMessage(`"code-v2"`), "input": json.RawMessage(`[{"role":"user","content":"hello"}]`),
					"stream": json.RawMessage(`true`), "previous_response_id": json.RawMessage(`"resp_previous"`),
					"vendor": json.RawMessage(`{"large":9007199254740993}`),
				}
				if diff := cmp.Diff(want, fields); diff != "" {
					t.Errorf("Responses upstream body mismatch (-want +got):\n%s", diff)
				}

				const target = "ai/target/96914ad662f712bc8376b8a6ec1aee8de3bb4f51dcd55f0670220c8fbe183d07"
				if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" || r.URL.RawQuery != "" || r.Header.Get(aidataplane.TargetHeader) != target {
					t.Errorf("Responses upstream method/path/target = %s/%s/%q, want POST /v1/responses and selected target", r.Method, r.URL.Path, r.Header.Get(aidataplane.TargetHeader))
				}

				if r.Header.Get("Authorization") != "" || r.Header.Get(aidataplane.HostNameHeader) != "" || r.Header.Get(aidataplane.ListenerPortHeader) != "" {
					t.Errorf("Responses upstream retained client authentication or entry headers: %v", r.Header)
				}

				w.Header().Set("Content-Type", tt.contentType)
				w.Header().Set("X-Request-Id", "upstream-id")
				w.WriteHeader(tt.status)
				if _, err := io.WriteString(w, tt.body); err != nil {
					t.Errorf("WriteString(Responses upstream message) error = %v", err)
				}
			})
			peer := &httptest.Server{Listener: egress, Config: &http.Server{Handler: peerHandler}}
			peer.Start()
			t.Cleanup(peer.Close)
			ingress := unixListener(t, "ingress.sock")

			synctest.Test(t, func(t *testing.T) {
				httpServer, closeIdle, err := NewHTTPServer(ingress, loadedConfig(t), egress.Addr().String(), 2048, slog.Default())
				if err != nil {
					t.Fatalf("NewHTTPServer(Responses) error = %v", err)
				}

				t.Cleanup(closeIdle)
				body := `{"model":"coding","input":[{"role":"user","content":"hello"}],"stream":true,"previous_response_id":"resp_previous","vendor":{"large":9007199254740993}}`
				request := httptest.NewRequest(http.MethodPost, "/v1/responses?ignored=client", strings.NewReader(body))
				request.Header.Set(aidataplane.ListenerPortHeader, "8080")
				request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")
				request.Header.Set(aidataplane.TargetHeader, "spoofed")
				request.Header.Set("Authorization", "Bearer client-credential")
				response := httptest.NewRecorder()

				httpServer.ServeHTTP(response, request)

				if response.Code != tt.status || response.Body.String() != tt.body || calls.Load() != 1 {
					t.Errorf("Responses(%s) = (%d, %q, calls %d), want (%d, %q, calls 1)", tt.name, response.Code, response.Body, calls.Load(), tt.status, tt.body)
				}

				if response.Header().Get("Content-Type") != tt.contentType || response.Header().Get("X-Request-Id") != "upstream-id" {
					t.Errorf("Responses(%s) headers = %v, want unchanged end-to-end fields", tt.name, response.Header())
				}
			})
		})
	}
}

func TestResponsesPipelineDoesNotExposeOtherOperations(t *testing.T) {
	ingress := unixListener(t, "ingress.sock")
	synctest.Test(t, func(t *testing.T) {
		httpServer, closeIdle, err := NewHTTPServer(ingress, loadedConfig(t), "/unused/egress.sock", 128, slog.Default())
		if err != nil {
			t.Fatalf("NewHTTPServer(unregistered Responses operations) error = %v", err)
		}

		t.Cleanup(closeIdle)
		for _, tt := range []struct {
			method string
			path   string
		}{
			{method: http.MethodGet, path: "/v1/responses"},
			{method: http.MethodGet, path: "/v1/responses/resp_1"},
			{method: http.MethodDelete, path: "/v1/responses/resp_1"},
			{method: http.MethodPost, path: "/v1/responses/resp_1/cancel"},
			{method: http.MethodPost, path: "/v1/responses/input_tokens"},
		} {
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"model":"coding"}`))
			request.Header.Set(aidataplane.ListenerPortHeader, "8080")
			request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")
			response := httptest.NewRecorder()

			httpServer.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Errorf("Responses(%s %s) status = %d, want 404 without forwarding", tt.method, tt.path, response.Code)
			}
		}
	})
}

func TestResponsesPipelineRejectsInvalidRequests(t *testing.T) {
	ingress := unixListener(t, "ingress.sock")
	synctest.Test(t, func(t *testing.T) {
		// 出口不存在；若拒绝输入错误地被转发，会得到 502 而非以下稳定的本地结果。
		httpServer, closeIdle, err := NewHTTPServer(ingress, loadedConfig(t), "/unused/egress.sock", 128, slog.Default())
		if err != nil {
			t.Fatalf("NewHTTPServer(Responses rejection) error = %v", err)
		}

		t.Cleanup(closeIdle)
		for _, tt := range []struct {
			name   string
			body   string
			code   string
			status int
		}{
			{name: "unknown model", body: `{"model":"other"}`, code: "MODEL_NOT_FOUND", status: 404},
			{name: "invalid model", body: `{"model":23}`, code: "MODEL_INVALID_ARGUMENT", status: 400},
			{name: "background task", body: `{"model":"coding","background":true}`, code: "MODEL_INVALID_ARGUMENT", status: 400},
			{name: "body limit", body: strings.Repeat("x", 129), code: "MODEL_REQUEST_TOO_LARGE", status: 413},
		} {
			request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(tt.body))
			request.Header.Set(aidataplane.ListenerPortHeader, "8080")
			request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")
			response := httptest.NewRecorder()

			httpServer.ServeHTTP(response, request)

			var result openai.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("Unmarshal(Responses %s error) = %v, want local model error", tt.name, err)
			}

			if response.Code != tt.status || result.Error.Code == nil || *result.Error.Code != tt.code {
				t.Errorf("Responses(%s) error = %d/%+v, want %d/%s", tt.name, response.Code, result.Error, tt.status, tt.code)
			}
		}
	})
}
