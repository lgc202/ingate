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
	"github.com/lgc202/ingate/internal/aiproxy/pkg/apischema/openai"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/modelproxy"
	modelservice "github.com/lgc202/ingate/internal/aiproxy/service/model"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

type failingBody struct {
	err   error
	reads int
}

func TestModelHandlerChatCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t, modelConfigMap(t))
		var forwarded *http.Request
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			forwarded = r
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll(forwarded body) error = %v", err)
			}
			var parsed openai.ChatCompletionRequest
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Fatalf("Unmarshal(forwarded body) error = %v", err)
			}
			if parsed.Model != "code-v2" {
				t.Errorf("ChatCompletion() forwarded model = %q, want code-v2", parsed.Model)
			}
			if r.ContentLength != int64(len(body)) || len(r.TransferEncoding) != 0 || r.GetBody != nil {
				t.Errorf("ChatCompletion() forwarded framing = (length %d, encoding %v, replay %t), want rewritten length without original framing or replay",
					r.ContentLength, r.TransferEncoding, r.GetBody != nil)
			}
			if len(r.Trailer) != 0 || r.Header.Get("Content-Length") != "" || r.Header.Get("Trailer") != "" {
				t.Errorf("ChatCompletion() retained original framing headers or trailers: %v, %v", r.Header, r.Trailer)
			}
			if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Content-Encoding") != "" {
				t.Errorf("ChatCompletion() body headers = %v, want uncompressed application/json", r.Header)
			}
			backend, ok := modelproxy.Backend(r.Context())
			if !ok || backend.NetworkBackendName != "team-a/model-network" || backend.Schema.Prefix != "/v1" {
				t.Errorf("modelproxy.Backend() = (%+v, %t), want selected model-network and /v1", backend, ok)
			}
			backend.ModelNameOverride = "changed"
			again, _ := modelproxy.Backend(r.Context())
			if again.ModelNameOverride != "code-v2" {
				t.Errorf("modelproxy.Backend() after result mutation override = %q, want code-v2", again.ModelNameOverride)
			}

			w.WriteHeader(http.StatusAccepted)
			if _, err := io.WriteString(w, `{"accepted":true}`); err != nil {
				t.Errorf("WriteString(response) error = %v", err)
			}
		})
		router := newModelRouter(t, service, next, 1024)
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"coding"}`))
		request = request.WithContext(WithModelEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
		request.Host = "untrusted.example"
		request.Header.Set("x-ai-eg-model", "other")
		request.Header.Set("Content-Length", "999")
		request.Header.Set("Trailer", "Digest")
		request.Header.Set("Content-Encoding", "identity")
		request.TransferEncoding = []string{"chunked"}
		request.Trailer = http.Header{"Digest": {"original-digest"}}
		request.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("original body")), nil }
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if forwarded == nil {
			t.Fatal("ChatCompletion() did not call the next stage, want prepared request")
		}
		if response.Code != http.StatusAccepted || response.Body.String() != `{"accepted":true}` {
			t.Errorf("ChatCompletion() response = (%d, %s), want unchanged next-stage response", response.Code, response.Body)
		}
		if request.Header.Get("Content-Length") != "999" || request.GetBody == nil || len(request.Trailer) != 1 || len(request.TransferEncoding) != 1 {
			t.Errorf("ChatCompletion() mutated incoming request framing: %+v", request)
		}
		if _, ok := modelproxy.Backend(request.Context()); ok {
			t.Error("ChatCompletion() changed incoming context, want target only on forwarded request")
		}
	})
}

func TestModelHandlerBodyLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t, modelConfigMap(t))
		body := `{"model":"coding"}`
		for _, tt := range []struct {
			name        string
			limit       int64
			wantStatus  int
			wantForward bool
		}{
			{name: "exact limit", limit: int64(len(body)), wantStatus: http.StatusNoContent, wantForward: true},
			{name: "over limit", limit: int64(len(body) - 1), wantStatus: http.StatusRequestEntityTooLarge},
		} {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})
			router := newModelRouter(t, service, next, tt.limit)
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			request = request.WithContext(WithModelEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
			request.ContentLength = -1 // 不能通过缺少或伪报 Content-Length 绕过实际读取上限。
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != tt.wantStatus || called != tt.wantForward {
				t.Errorf("ChatCompletion(%s) = (status %d, forwarded %t), want (%d, %t)", tt.name, response.Code, called, tt.wantStatus, tt.wantForward)
			}
		}
	})
}

func TestModelHandlerRejectsWithoutForwarding(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t, modelConfigMap(t))
		for _, tt := range []struct {
			name       string
			body       string
			port       int32
			host       string
			wantStatus int
		}{
			{name: "invalid JSON", body: `{"model":`, port: 8080, host: "gateway/team-a/public/listener/api", wantStatus: http.StatusBadRequest},
			{name: "unknown model", body: `{"model":"other"}`, port: 8080, host: "gateway/team-a/public/listener/api", wantStatus: http.StatusNotFound},
			{name: "other namespace", body: `{"model":"coding"}`, port: 8080, host: "gateway/team-b/public/listener/api", wantStatus: http.StatusNotFound},
			{name: "other port", body: `{"model":"coding"}`, port: 8081, host: "gateway/team-a/public/listener/api", wantStatus: http.StatusNotFound},
		} {
			called := false
			router := newModelRouter(t, service, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), 1024)
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tt.body))
			request = request.WithContext(WithModelEntry(t.Context(), tt.port, tt.host))
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != tt.wantStatus || called {
				t.Errorf("ChatCompletion(%s) = (status %d, forwarded %t), want (%d, false)", tt.name, response.Code, called, tt.wantStatus)
			}
			if strings.Contains(response.Body.String(), tt.host) || strings.Contains(response.Body.String(), tt.body) {
				t.Errorf("ChatCompletion(%s) exposed internal entry or request in %s", tt.name, response.Body)
			}
		}
	})
}

func TestModelHandlerRequiresTrustedEntry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t, modelConfigMap(t))
		called := false
		router := newModelRouter(t, service, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), 1024)
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"coding"}`))
		request.Header.Set("x-ingate-ai-listener-port", "8080")
		request.Header.Set("x-ingate-ai-host", "gateway/team-a/public/listener/api")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusServiceUnavailable || called {
			t.Errorf("ChatCompletion(client-declared entry) = (status %d, forwarded %t), want (503, false)", response.Code, called)
		}
	})
}

func TestModelHandlerBeforeConfigurationLoad(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t)
		called := false
		router := newModelRouter(t, service, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), 1024)
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"coding"}`))
		request = request.WithContext(WithModelEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusServiceUnavailable || called {
			t.Errorf("ChatCompletion(before configuration load) = (status %d, forwarded %t), want (503, false)", response.Code, called)
		}
	})
}

func TestModelHandlerPreservesResponseWriter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t, modelConfigMap(t))
		response := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if w != response {
				t.Errorf("ChatCompletion() writer = %T, want original recorder", w)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			if _, err := io.WriteString(w, "data: first\n\n"); err != nil {
				t.Errorf("WriteString(first event) error = %v", err)
			}
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Errorf("Flush(first event) error = %v", err)
			}
			if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
				t.Errorf("WriteString(final event) error = %v", err)
			}
		})
		router := newModelRouter(t, service, next, 1024)
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"coding","stream":true}`))
		request = request.WithContext(WithModelEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))

		router.ServeHTTP(response, request)

		if !response.Flushed || response.Body.String() != "data: first\n\ndata: [DONE]\n\n" {
			t.Errorf("ChatCompletion(stream) = (flushed %t, body %q), want unchanged events and flush", response.Flushed, response.Body.String())
		}
	})
}

func TestModelHandlerMiddlewareRunsBeforeRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t, modelConfigMap(t))
		called := false
		deny := func(middleware.Handler) middleware.Handler {
			return func(context.Context, any) (any, error) {
				return nil, kratoserrors.Forbidden("TEST_DENIED", "access denied")
			}
		}
		next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
		router := newModelRouter(t, service, next, 1024, kratoshttp.Middleware(deny))
		body := &failingBody{err: errors.New("must not read unauthorized body")}
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		request.Body = body
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusForbidden || called || body.reads != 0 {
			t.Errorf("ChatCompletion(denied middleware) = (status %d, forwarded %t, reads %d), want (403, false, 0)", response.Code, called, body.reads)
		}
	})
}

func TestModelHandlerPreservesMiddlewareContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t, modelConfigMap(t))
		entry := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, request any) (any, error) {
				return next(WithModelEntry(ctx, 8080, "gateway/team-a/public/listener/api"), request)
			}
		}
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			if _, ok := modelproxy.Backend(r.Context()); !ok {
				t.Error("modelproxy.Backend(forwarded middleware context) is absent, want prepared target")
			}
			if _, ok := r.Context().Value(modelEntryKey{}).(modelEntry); !ok {
				t.Error("ChatCompletion() lost the entry context added by middleware")
			}
			w.WriteHeader(http.StatusNoContent)
		})
		router := newModelRouter(t, service, next, 1024, kratoshttp.Middleware(entry))
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"coding"}`))
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusNoContent || !called {
			t.Errorf("ChatCompletion(middleware entry) = (status %d, forwarded %t), want (204, true)", response.Code, called)
		}
	})
}

func TestModelHandlerReadErrorDoesNotExposeDetails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t, modelConfigMap(t))
		called := false
		next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
		router := newModelRouter(t, service, next, 1024)
		body := &failingBody{err: errors.New("sensitive request details")}
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		request.Body = body
		request = request.WithContext(WithModelEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest || called || body.reads == 0 {
			t.Errorf("ChatCompletion(read failure) = (status %d, forwarded %t, reads %d), want (400, false, positive reads)", response.Code, called, body.reads)
		}
		if strings.Contains(response.Body.String(), "sensitive") {
			t.Errorf("ChatCompletion(read failure) exposed private details in %s", response.Body)
		}
	})
}

func TestModelHandlerRejectsCompressedBody(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t, modelConfigMap(t))
		called := false
		next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
		router := newModelRouter(t, service, next, 1024)
		body := &failingBody{err: errors.New("must not parse compressed body as plaintext")}
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		request.Body = body
		request.Header.Add("Content-Encoding", "identity")
		request.Header.Add("Content-Encoding", "gzip")
		request = request.WithContext(WithModelEntry(t.Context(), 8080, "gateway/team-a/public/listener/api"))
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusUnsupportedMediaType || called || body.reads != 0 {
			t.Errorf("ChatCompletion(compressed body) = (status %d, forwarded %t, reads %d), want (415, false, 0)", response.Code, called, body.reads)
		}
	})
}

func TestModelHandlerCancelledBeforeRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := startModelService(t, modelConfigMap(t))
		called := false
		next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
		router := newModelRouter(t, service, next, 1024)
		body := &failingBody{err: errors.New("must not read cancelled request")}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		request.Body = body
		request = request.WithContext(ctx)
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != 499 || called || body.reads != 0 {
			t.Errorf("ChatCompletion(cancelled request) = (status %d, forwarded %t, reads %d), want (499, false, 0)", response.Code, called, body.reads)
		}
	})
}

func TestNewModelRejectsInvalidDependencies(t *testing.T) {
	loader, err := aiconfig.NewLoader(fake.NewClientset(), "control-plane", slog.Default())
	if err != nil {
		t.Fatalf("NewLoader(constructor test) error = %v", err)
	}
	service := modelservice.New(loader)
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, tt := range []struct {
		name    string
		service *modelservice.Service
		next    http.Handler
		limit   int64
	}{
		{name: "missing service", next: next, limit: 1},
		{name: "missing next", service: service, limit: 1},
		{name: "zero limit", service: service, next: next},
		{name: "negative limit", service: service, next: next, limit: -1},
	} {
		handler, err := NewModel(tt.service, tt.next, tt.limit)

		if handler != nil || err == nil {
			t.Errorf("NewModel(%s) = (%+v, %v), want nil handler and error", tt.name, handler, err)
		}
	}
}

func TestModelTransportError(t *testing.T) {
	for _, tt := range []struct {
		name       string
		cause      error
		wantStatus int
	}{
		{name: "invalid argument", cause: modelservice.ErrInvalidArgument, wantStatus: http.StatusBadRequest},
		{name: "model not found", cause: modelservice.ErrModelNotFound, wantStatus: http.StatusNotFound},
		{name: "unavailable", cause: modelservice.ErrUnavailable, wantStatus: http.StatusServiceUnavailable},
		{name: "cancelled", cause: context.Canceled, wantStatus: 499},
		{name: "deadline", cause: context.DeadlineExceeded, wantStatus: http.StatusGatewayTimeout},
		{
			name: "unknown transport error",
			cause: kratoserrors.BadRequest("PRIVATE", "sensitive upstream detail").
				WithMetadata(map[string]string{"secret": "private metadata"}),
			wantStatus: http.StatusInternalServerError,
		},
	} {
		err := fmt.Errorf("private detail: %w", tt.cause)

		mapped := toModelTransportError(t.Context(), err)

		if kratoserrors.Code(mapped) != tt.wantStatus || !errors.Is(mapped, tt.cause) {
			t.Errorf("toModelTransportError(%s) = %v, want status %d with original cause", tt.name, mapped, tt.wantStatus)
		}
		if strings.Contains(kratoserrors.FromError(mapped).Message, "private") || strings.Contains(kratoserrors.FromError(mapped).Message, "sensitive") {
			t.Errorf("toModelTransportError(%s) exposed private detail in public message", tt.name)
		}

		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

		kratoshttp.DefaultErrorEncoder(response, request, mapped)

		if response.Code != tt.wantStatus {
			t.Errorf("DefaultErrorEncoder(%s) status = %d, want %d", tt.name, response.Code, tt.wantStatus)
		}
		encodedBody := response.Body.String()
		if strings.Contains(encodedBody, "private") || strings.Contains(encodedBody, "sensitive") || strings.Contains(encodedBody, "secret") {
			t.Errorf("DefaultErrorEncoder(%s) = (%d, %s), want status %d without private cause or metadata", tt.name, response.Code, response.Body, tt.wantStatus)
		}
	}
}

func (b *failingBody) Read([]byte) (int, error) {
	b.reads++

	return 0, b.err
}

func (b *failingBody) Close() error {
	return nil
}

func newModelRouter(
	t *testing.T,
	service *modelservice.Service,
	next http.Handler,
	limit int64,
	options ...kratoshttp.ServerOption,
) *kratoshttp.Server {
	t.Helper()
	handler, err := NewModel(service, next, limit)
	if err != nil {
		t.Fatalf("NewModel(test router) error = %v", err)
	}
	options = append([]kratoshttp.ServerOption{kratoshttp.Timeout(0)}, options...)
	router := kratoshttp.NewServer(options...)
	router.Route("/").POST("/v1/chat/completions", handler.ChatCompletion)

	return router
}

// startModelService 必须在 synctest 中调用；测试结束时取消并等待真实 Loader 返回。
func startModelService(t *testing.T, objects ...runtime.Object) *modelservice.Service {
	t.Helper()
	client := fake.NewClientset(objects...)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader, err := aiconfig.NewLoader(client, "control-plane", logger)
	if err != nil {
		t.Fatalf("NewLoader(test model handler) error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- loader.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run(stopped test loader) error = %v, want context.Canceled", err)
		}
	})
	synctest.Wait()

	return modelservice.New(loader)
}

func modelConfigMap(t *testing.T) *corev1.ConfigMap {
	t.Helper()
	data, err := os.ReadFile("../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(configuration fixture) error = %v", err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "control-plane",
			Name:            aidataplane.ConfigMapName,
			UID:             "original",
			ResourceVersion: "1",
		},
		Data: map[string]string{aidataplane.ConfigMapDataKey: string(data)},
	}
}
