package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/go-cmp/cmp"

	"github.com/lgc202/ingate/internal/aiproxy/errno"
)

func TestModelErrorEncoder(t *testing.T) {
	for _, tt := range []struct {
		name    string
		err     error
		status  int
		kind    string
		code    string
		message string
	}{
		{name: "invalid request", err: errno.ErrModelInvalidArgument, status: 400, kind: "invalid_request_error", code: "MODEL_INVALID_ARGUMENT", message: "invalid model request"},
		{name: "missing model", err: errno.ErrModelNotFound, status: 404, kind: "invalid_request_error", code: "MODEL_NOT_FOUND", message: "model is not available at this entry"},
		{name: "body limit", err: errno.ErrModelRequestTooLarge, status: 413, kind: "invalid_request_error", code: "MODEL_REQUEST_TOO_LARGE", message: "model request body is too large"},
		{name: "encoding", err: errno.ErrModelEncodingUnsupported, status: 415, kind: "invalid_request_error", code: "MODEL_ENCODING_UNSUPPORTED", message: "compressed model requests are not supported"},
		{name: "configuration", err: errno.ErrModelUnavailable, status: 503, kind: "server_error", code: "MODEL_UNAVAILABLE", message: "model service is unavailable"},
		{
			name:    "forwarding",
			err:     errno.ErrModelForwardFailed.WithCause(errors.New("private socket path")).WithMetadata(map[string]string{"secret": "private metadata"}),
			status:  502,
			kind:    "server_error",
			code:    "MODEL_FORWARD_FAILED",
			message: "model forwarding failed",
		},
		{name: "deadline", err: errno.ErrDeadlineExceeded, status: 504, kind: "server_error", code: "DEADLINE_EXCEEDED", message: "request deadline exceeded"},
		{name: "canceled", err: errno.ErrRequestCanceled, status: 499, kind: "invalid_request_error", code: "REQUEST_CANCELED", message: "request was canceled"},
		{name: "wrapped public error", err: fmt.Errorf("private context: %w", errno.ErrModelInvalidArgument), status: 400, kind: "invalid_request_error", code: "MODEL_INVALID_ARGUMENT", message: "invalid model request"},
		{name: "unknown error", err: errors.New("private implementation detail"), status: 500, kind: "server_error", code: "INTERNAL_SERVER_ERROR", message: "internal server error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// 通过实际 Server 触发编码器，避免只验证未接入的辅助函数。
			httpServer := newHTTP(testUnixListener(t))
			httpServer.Route("/").POST("/error", func(kratoshttp.Context) error { return tt.err })
			request := httptest.NewRequest(http.MethodPost, "/error", nil)
			request.Header.Set("Accept", "application/xml")
			response := httptest.NewRecorder()

			httpServer.ServeHTTP(response, request)

			if response.Code != tt.status || response.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("error response status/type = %d/%q, want %d/application/json", response.Code, response.Header().Get("Content-Type"), tt.status)
			}

			want := map[string]any{"error": map[string]any{
				"message": tt.message, "type": tt.kind, "param": nil, "code": tt.code,
			}}
			if diff := cmp.Diff(want, decodeModelError(t, response)); diff != "" {
				t.Errorf("error response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func testUnixListener(t *testing.T) *net.UnixListener {
	t.Helper()

	// macOS 的 Unix 地址长度有限，测试框架的默认临时目录可能过长。
	directory, err := os.MkdirTemp("/tmp", "ingate-error-")
	if err != nil {
		t.Fatalf("create socket directory: %v", err)
	}

	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Errorf("remove socket directory: %v", err)
		}
	})
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Net: "unix", Name: filepath.Join(directory, "ingress.sock")})
	if err != nil {
		t.Fatalf("listen Unix socket: %v", err)
	}

	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close Unix listener: %v", err)
		}
	})

	return listener
}

func decodeModelError(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	decoder := json.NewDecoder(response.Body)
	var result map[string]any
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("decode model error: %v", err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("model error trailing content = %v, want EOF", err)
	}

	return result
}
