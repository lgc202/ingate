package handler

import (
	"bytes"
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
	"testing"
	"testing/synctest"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/configtest"
	"github.com/lgc202/ingate/internal/aiproxy/service/modelproxy"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

type failingBody struct {
	err   error
	reads int
}

func TestModelProxyHandlerChatCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		called := false
		transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			var fields map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
				t.Errorf("Decode(outbound body) error = %v", err)
			}

			if string(fields["model"]) != `"code-v2"` {
				t.Errorf("ChatCompletion() outbound model = %s, want code-v2", fields["model"])
			}

			if r.Header.Get(aidataplane.TargetHeader) == "spoofed" || r.Header.Get(aidataplane.TargetHeader) == "" {
				t.Errorf("ChatCompletion() target = %q, want generated target", r.Header.Get(aidataplane.TargetHeader))
			}

			for _, name := range []string{
				"Authorization", "Cookie", "X-Api-Key", "X-Custom-Secret", "Forwarded", "X-Forwarded-For",
				"X-Envoy-Upstream-Rq-Timeout-Ms", "X-Ai-Eg-Model", "Content-Length", "Trailer",
			} {
				if r.Header.Get(name) != "" {
					t.Errorf("ChatCompletion() retained client header %q", name)
				}
			}

			if r.URL.Host != "ingate-ai-egress" || r.URL.RawQuery != "" || r.GetBody != nil || len(r.TransferEncoding) != 0 || len(r.Trailer) != 0 {
				t.Errorf("ChatCompletion() outbound request = %s, want fixed fresh request without replay or framing inheritance", r.URL)
			}

			if r.Header.Get("Accept") != "text/event-stream" || r.Header.Get("User-Agent") != "" {
				t.Errorf("ChatCompletion() Accept/User-Agent = %q/%q, want allowed negotiation and no client identity", r.Header.Get("Accept"), r.Header.Get("User-Agent"))
			}

			return modelResponse(http.StatusAccepted, `{"accepted":true}`, nil), nil
		})
		service := startModelService(t, transport, modelConfig(t))
		router := newModelRouter(t, service, 1024)
		request := trustedModelRequest(t, `{"model":"coding"}`)
		request.URL.Host = "untrusted.example"
		request.URL.RawQuery = "key=secret"
		request.Header.Set("Authorization", "private credential")
		request.Header.Set("Cookie", "private cookie")
		request.Header.Set("X-Api-Key", "private key")
		request.Header.Set("X-Custom-Secret", "private value")
		request.Header.Set("Forwarded", "for=untrusted")
		request.Header.Set("X-Forwarded-For", "untrusted")
		request.Header.Set("X-Envoy-Upstream-Rq-Timeout-Ms", "1")
		request.Header.Set("X-Ai-Eg-Model", "spoofed")
		request.Header.Set(aidataplane.TargetHeader, "spoofed")
		request.Header.Set("Accept", "text/event-stream")
		request.Header.Set("User-Agent", "private client identity")
		request.Header.Set("Content-Length", "999")
		request.Header.Set("Trailer", "Digest")
		request.Trailer = http.Header{"Digest": {"original"}}
		request.TransferEncoding = []string{"chunked"}
		request.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("original")), nil }
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if !called || response.Code != http.StatusAccepted || response.Body.String() != `{"accepted":true}` {
			t.Errorf("ChatCompletion() = (called %t, status %d, body %s), want original upstream response", called, response.Code, response.Body)
		}

		if request.URL.Host != "untrusted.example" || request.Header.Get("Content-Length") != "999" || request.GetBody == nil || len(request.Trailer) != 1 {
			t.Error("ChatCompletion() modified the original request metadata")
		}
	})
}

func TestModelProxyHandlerRejectsBeforeClientCall(t *testing.T) {
	for _, tt := range []struct {
		name       string
		body       string
		limit      int64
		trusted    bool
		wantStatus int
	}{
		{name: "missing entry", body: `{"model":"coding"}`, limit: 1024, wantStatus: http.StatusServiceUnavailable},
		{name: "unknown model", body: `{"model":"other"}`, limit: 1024, trusted: true, wantStatus: http.StatusNotFound},
		{name: "invalid JSON", body: `{`, limit: 1024, trusted: true, wantStatus: http.StatusBadRequest},
		{name: "over limit", body: `{"model":"coding"}`, limit: 1, trusted: true, wantStatus: http.StatusRequestEntityTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				called := false
				transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return modelResponse(http.StatusNoContent, "", nil), nil
				})
				service := startModelService(t, transport, modelConfig(t))
				router := newModelRouter(t, service, tt.limit)
				request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tt.body))
				request.Header.Set("x-ingate-ai-listener-port", "8080")
				request.Header.Set("x-ingate-ai-host", "gateway/team-a/public/listener/api")
				if tt.trusted {
					request = request.WithContext(WithEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
				}

				response := httptest.NewRecorder()

				router.ServeHTTP(response, request)

				if response.Code != tt.wantStatus || called {
					t.Errorf("ChatCompletion(%s) status/called = %d/%t, want %d/false", tt.name, response.Code, called, tt.wantStatus)
				}
			})
		})
	}
}

func TestModelProxyHandlerExactBodyLimitAndUnloadedConfiguration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return modelResponse(http.StatusNoContent, "", nil), nil
		})
		body := `{"model":"coding"}`
		service := startModelService(t, transport, modelConfig(t))
		router := newModelRouter(t, service, int64(len(body)))
		response := httptest.NewRecorder()

		router.ServeHTTP(response, trustedModelRequest(t, body))

		if response.Code != http.StatusNoContent || calls != 1 {
			t.Errorf("ChatCompletion(exact limit) status/calls = %d/%d, want 204/1", response.Code, calls)
		}

		unloaded := startModelService(t, transport)
		router = newModelRouter(t, unloaded, 1024)
		response = httptest.NewRecorder()

		router.ServeHTTP(response, trustedModelRequest(t, body))

		if response.Code != http.StatusServiceUnavailable || calls != 1 {
			t.Errorf("ChatCompletion(unloaded) status/calls = %d/%d, want 503 without another call", response.Code, calls)
		}
	})
}

func TestModelProxyHandlerMiddlewarePrecedesReadAndPreservesContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		called := false
		transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			if _, ok := r.Context().Value(entryKey{}).(entry); !ok {
				t.Error("ChatCompletion() lost the middleware-derived request context")
			}

			return modelResponse(http.StatusNoContent, "", nil), nil
		})
		service := startModelService(t, transport, modelConfig(t))
		deny := func(middleware.Handler) middleware.Handler {
			return func(context.Context, any) (any, error) {
				return nil, kratoserrors.Forbidden("TEST_DENIED", "access denied")
			}
		}
		router := newModelRouter(t, service, 1024, kratoshttp.Middleware(deny))
		body := &failingBody{err: errors.New("must not read unauthorized body")}
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		request.Body = body
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusForbidden || called || body.reads != 0 {
			t.Errorf("ChatCompletion(denied) status/called/reads = %d/%t/%d, want 403/false/0", response.Code, called, body.reads)
		}

		withEntry := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, request any) (any, error) {
				return next(WithEntry(ctx, 8080, "gateway/team-a/public/listener/api"), request)
			}
		}
		router = newModelRouter(t, service, 1024, kratoshttp.Middleware(withEntry))
		response = httptest.NewRecorder()

		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"coding"}`)))

		if response.Code != http.StatusNoContent || !called {
			t.Errorf("ChatCompletion(middleware entry) status/called = %d/%t, want 204/true", response.Code, called)
		}
	})
}

func TestModelProxyHandlerReadFailures(t *testing.T) {
	for _, mode := range []string{"read error", "compressed", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				called := false
				transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return modelResponse(http.StatusNoContent, "", nil), nil
				})
				service := startModelService(t, transport, modelConfig(t))
				router := newModelRouter(t, service, 1024)
				body := &failingBody{err: errors.New("sensitive request details")}
				request := trustedModelRequest(t, "")
				request.Body = body
				wantStatus, wantReads := http.StatusBadRequest, 1

				switch mode {
				case "compressed":
					request.Header.Add("Content-Encoding", "identity")
					request.Header.Add("Content-Encoding", "gzip")
					wantStatus, wantReads = http.StatusUnsupportedMediaType, 0
				case "canceled":
					ctx, cancel := context.WithCancel(request.Context())
					cancel()
					request = request.WithContext(ctx)
					wantStatus, wantReads = 499, 0
				}

				response := httptest.NewRecorder()

				router.ServeHTTP(response, request)

				if response.Code != wantStatus || called || body.reads != wantReads || strings.Contains(response.Body.String(), "sensitive") {
					t.Errorf("ChatCompletion(%s) status/called/reads/body = %d/%t/%d/%s, want safe %d/false/%d", mode, response.Code, called, body.reads, response.Body, wantStatus, wantReads)
				}
			})
		})
	}
}

func TestNewModelProxyRejectsInvalidBodyLimit(t *testing.T) {
	loader := aiconfig.NewLoader(nil, "test", slog.Default())

	service := modelproxy.New(loader, &http.Client{})
	for _, limit := range []int64{0, -1} {
		modelProxyHandler, err := NewModelProxy(service, limit, slog.Default())

		if modelProxyHandler != nil || err == nil {
			t.Errorf("NewModelProxy(limit %d) = (%v, %v), want nil and configuration error", limit, modelProxyHandler, err)
		}
	}
}

func TestModelTransportError(t *testing.T) {
	for _, tt := range []struct {
		name        string
		cause       error
		wantStatus  int
		wantReason  string
		wantMessage string
	}{
		{
			name:        "invalid",
			cause:       modelproxy.ErrInvalidArgument,
			wantStatus:  400,
			wantReason:  "MODEL_INVALID_ARGUMENT",
			wantMessage: "invalid model request",
		},
		{
			name:        "not found",
			cause:       modelproxy.ErrModelNotFound,
			wantStatus:  404,
			wantReason:  "MODEL_NOT_FOUND",
			wantMessage: "model is not available at this entry",
		},
		{
			name:        "unavailable",
			cause:       modelproxy.ErrUnavailable,
			wantStatus:  503,
			wantReason:  "MODEL_UNAVAILABLE",
			wantMessage: "model service is unavailable",
		},
		{
			name:        "forward failed",
			cause:       modelproxy.ErrForwardFailed,
			wantStatus:  502,
			wantReason:  "MODEL_FORWARD_FAILED",
			wantMessage: "model forwarding failed",
		},
		{
			name:        "canceled",
			cause:       fmt.Errorf("%w: %w", modelproxy.ErrForwardFailed, context.Canceled),
			wantStatus:  499,
			wantReason:  "REQUEST_CANCELED",
			wantMessage: "request was canceled",
		},
		{
			name:        "deadline",
			cause:       fmt.Errorf("%w: %w", modelproxy.ErrForwardFailed, context.DeadlineExceeded),
			wantStatus:  504,
			wantReason:  "DEADLINE_EXCEEDED",
			wantMessage: "request deadline exceeded",
		},
		{
			name:        "unknown protocol error",
			cause:       kratoserrors.BadRequest("PRIVATE", "sensitive detail").WithMetadata(map[string]string{"secret": "private"}),
			wantStatus:  500,
			wantReason:  "INTERNAL_SERVER_ERROR",
			wantMessage: "internal server error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mapped := toModelTransportError(t.Context(), fmt.Errorf("private cause: %w", tt.cause), slog.Default())
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

			kratoshttp.DefaultErrorEncoder(response, request, mapped)

			if response.Code != tt.wantStatus || !errors.Is(mapped, tt.cause) {
				t.Errorf("toModelTransportError(%s) = (%d, %v), want %d with cause", tt.name, response.Code, mapped, tt.wantStatus)
			}

			var result struct {
				Code    int    `json:"code"`
				Reason  string `json:"reason"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode toModelTransportError(%s) response: %v", tt.name, err)
			}

			if result.Code != tt.wantStatus || result.Reason != tt.wantReason || result.Message != tt.wantMessage {
				t.Errorf("toModelTransportError(%s) response = %+v, want %d/%s/%s", tt.name, result, tt.wantStatus, tt.wantReason, tt.wantMessage)
			}

			for _, forbidden := range []string{"sensitive", "private", "secret", "PRIVATE"} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Errorf("toModelTransportError(%s) exposed %q in %s", tt.name, forbidden, response.Body)
				}
			}
		})
	}
}

func TestModelTransportErrorUsesInjectedLoggerOnce(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil)).With("service", "ai-proxy-test")
	cause := errors.New("private failure")

	mapped := toModelTransportError(t.Context(), cause, logger)

	if !errors.Is(mapped, cause) {
		t.Errorf("toModelTransportError(unknown failure) = %v, want preserved cause", mapped)
	}

	decoder := json.NewDecoder(&output)
	var record map[string]any
	if err := decoder.Decode(&record); err != nil {
		t.Fatalf("Decode(injected error log) error = %v", err)
	}

	if record["service"] != "ai-proxy-test" || record["err"] != "private failure" {
		t.Errorf("toModelTransportError(unknown failure) log = %v, want injected service and cause", record)
	}

	if err := decoder.Decode(&record); !errors.Is(err, io.EOF) {
		t.Errorf("toModelTransportError(unknown failure) extra log = %v, want exactly one record", err)
	}

	output.Reset()

	mapped = toModelTransportError(t.Context(), context.Canceled, logger)

	if !errors.Is(mapped, context.Canceled) {
		t.Errorf("toModelTransportError(canceled) = %v, want preserved cancellation", mapped)
	}

	if output.Len() != 0 {
		t.Errorf("toModelTransportError(canceled) wrote %q, want no request error log", output.String())
	}
}

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	// Service 的正文来自 bytes.Reader，原生 NewRequest 包装的 Close 无副作用且始终成功。
	defer func() { _ = request.Body.Close() }()

	return f(request)
}

func (b *failingBody) Read([]byte) (int, error) {
	b.reads++

	return 0, b.err
}

func (b *failingBody) Close() error { return nil }

func newModelRouter(
	t *testing.T,
	service *modelproxy.Service,
	limit int64,
	options ...kratoshttp.ServerOption,
) *kratoshttp.Server {
	t.Helper()
	modelProxyHandler, err := NewModelProxy(service, limit, slog.Default())
	if err != nil {
		t.Fatalf("NewModelProxy(test router) error = %v", err)
	}

	options = append([]kratoshttp.ServerOption{kratoshttp.Timeout(0)}, options...)
	router := kratoshttp.NewServer(options...)
	router.Route("/").POST("/v1/chat/completions", modelProxyHandler.ChatCompletion)

	return router
}

func startModelService(t *testing.T, transport http.RoundTripper, configs ...*aidataplane.Config) *modelproxy.Service {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if len(configs) == 0 {
		return modelproxy.New(aiconfig.NewLoader(nil, "test", logger), &http.Client{Transport: transport})
	}

	source := configtest.New(t)
	source.Publish(t, *configs[0])
	loader := aiconfig.NewLoader(source.Client, "test", logger)

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

	return modelproxy.New(loader, &http.Client{Transport: transport})
}

func trustedModelRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))

	return request.WithContext(WithEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
}

func modelResponse(status int, body string, headers http.Header) *http.Response {
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
}

func modelConfig(t *testing.T) *aidataplane.Config {
	t.Helper()
	data, err := os.ReadFile("../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(configuration fixture) error = %v", err)
	}

	config, err := aidataplane.Decode(data)
	if err != nil {
		t.Fatalf("Decode(configuration fixture) error = %v", err)
	}

	return config
}
