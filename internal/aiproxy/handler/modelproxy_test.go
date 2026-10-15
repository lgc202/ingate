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
	"os"
	"strings"
	"testing"
	"testing/synctest"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
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
		service := startModelService(t, transport, modelConfigMap(t))
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
				service := startModelService(t, transport, modelConfigMap(t))
				router := newModelRouter(t, service, tt.limit)
				request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tt.body))
				request.Header.Set("x-ingate-ai-listener-port", "8080")
				request.Header.Set("x-ingate-ai-host", "gateway/team-a/public/listener/api")
				if tt.trusted {
					request = request.WithContext(WithModelEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
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
		service := startModelService(t, transport, modelConfigMap(t))
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
			if _, ok := r.Context().Value(modelEntryKey{}).(modelEntry); !ok {
				t.Error("ChatCompletion() lost the middleware-derived request context")
			}
			return modelResponse(http.StatusNoContent, "", nil), nil
		})
		service := startModelService(t, transport, modelConfigMap(t))
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

		entry := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, request any) (any, error) {
				return next(WithModelEntry(ctx, 8080, "gateway/team-a/public/listener/api"), request)
			}
		}
		router = newModelRouter(t, service, 1024, kratoshttp.Middleware(entry))
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
				service := startModelService(t, transport, modelConfigMap(t))
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
	loader, err := aiconfig.NewLoader(fake.NewClientset(), "control-plane", slog.Default())
	if err != nil {
		t.Fatalf("NewLoader(constructor test) error = %v", err)
	}
	service := modelproxy.New(loader, &http.Client{})
	for _, limit := range []int64{0, -1} {
		modelProxyHandler, err := NewModelProxy(service, limit)

		if modelProxyHandler != nil || err == nil {
			t.Errorf("NewModelProxy(limit %d) = (%v, %v), want nil and configuration error", limit, modelProxyHandler, err)
		}
	}
}

func TestModelTransportError(t *testing.T) {
	for _, tt := range []struct {
		name       string
		cause      error
		wantStatus int
	}{
		{name: "invalid", cause: modelproxy.ErrInvalidArgument, wantStatus: 400},
		{name: "not found", cause: modelproxy.ErrModelNotFound, wantStatus: 404},
		{name: "unavailable", cause: modelproxy.ErrUnavailable, wantStatus: 503},
		{name: "forward failed", cause: modelproxy.ErrForwardFailed, wantStatus: 502},
		{name: "canceled", cause: fmt.Errorf("%w: %w", modelproxy.ErrForwardFailed, context.Canceled), wantStatus: 499},
		{name: "deadline", cause: fmt.Errorf("%w: %w", modelproxy.ErrForwardFailed, context.DeadlineExceeded), wantStatus: 504},
		{name: "unknown protocol error", cause: kratoserrors.BadRequest("PRIVATE", "sensitive detail").WithMetadata(map[string]string{"secret": "private"}), wantStatus: 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mapped := toModelTransportError(t.Context(), fmt.Errorf("private cause: %w", tt.cause))
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

			kratoshttp.DefaultErrorEncoder(response, request, mapped)

			if response.Code != tt.wantStatus || !errors.Is(mapped, tt.cause) {
				t.Errorf("toModelTransportError(%s) = (%d, %v), want %d with cause", tt.name, response.Code, mapped, tt.wantStatus)
			}
			for _, forbidden := range []string{"sensitive", "private", "secret", "PRIVATE"} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Errorf("toModelTransportError(%s) exposed %q in %s", tt.name, forbidden, response.Body)
				}
			}
		})
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
	modelProxyHandler, err := NewModelProxy(service, limit)
	if err != nil {
		t.Fatalf("NewModelProxy(test router) error = %v", err)
	}
	options = append([]kratoshttp.ServerOption{kratoshttp.Timeout(0)}, options...)
	router := kratoshttp.NewServer(options...)
	router.Route("/").POST("/v1/chat/completions", modelProxyHandler.ChatCompletion)

	return router
}

func startModelService(t *testing.T, transport http.RoundTripper, objects ...runtime.Object) *modelproxy.Service {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader, err := aiconfig.NewLoader(fake.NewClientset(objects...), "control-plane", logger)
	if err != nil {
		t.Fatalf("NewLoader(test handler) error = %v", err)
	}
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

	return request.WithContext(WithModelEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
}

func modelResponse(status int, body string, headers http.Header) *http.Response {
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
}

func modelConfigMap(t *testing.T) *corev1.ConfigMap {
	t.Helper()
	data, err := os.ReadFile("../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(configuration fixture) error = %v", err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "control-plane", Name: aidataplane.ConfigMapName, UID: "original", ResourceVersion: "1"},
		Data:       map[string]string{aidataplane.ConfigMapDataKey: string(data)},
	}
}
