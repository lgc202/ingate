package errno

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFallbackPublicContract(t *testing.T) {
	internal := kratoserrors.BadRequest("DOWNSTREAM_PRIVATE", "mysql://private-address").
		WithMetadata(map[string]string{"password": "private-secret"})

	for _, tt := range []struct {
		name     string
		cause    error
		code     int
		grpcCode codes.Code
		reason   string
		message  string
	}{
		{
			name:     "unknown downstream error",
			cause:    internal,
			code:     http.StatusInternalServerError,
			grpcCode: codes.Internal,
			reason:   "INTERNAL_SERVER_ERROR",
			message:  "服务器内部错误",
		},
		{
			name:     "wrapped cancellation",
			cause:    fmt.Errorf("read: %w", context.Canceled),
			code:     499,
			grpcCode: codes.Canceled,
			reason:   "REQUEST_CANCELED",
			message:  "请求已取消",
		},
		{
			name:     "wrapped deadline",
			cause:    fmt.Errorf("read: %w", context.DeadlineExceeded),
			code:     http.StatusGatewayTimeout,
			grpcCode: codes.DeadlineExceeded,
			reason:   "DEADLINE_EXCEEDED",
			message:  "请求处理超时",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := Fallback(tt.cause)
			public, ok := errors.AsType[*kratoserrors.Error](err)
			if !ok {
				t.Fatalf("Fallback(%v) = %T, want Kratos error", tt.cause, err)
			}

			if public.Code != int32(tt.code) || public.Reason != tt.reason || public.Message != tt.message {
				t.Errorf("Fallback(%v) = %v, want %d/%s/%s", tt.cause, public, tt.code, tt.reason, tt.message)
			}

			if !errors.Is(err, tt.cause) {
				t.Errorf("Fallback(%v) lost its cause", tt.cause)
			}

			response := httptest.NewRecorder()
			kratoshttp.DefaultErrorEncoder(response, httptest.NewRequest(http.MethodGet, "/", nil), err)
			if response.Code != tt.code || strings.Contains(response.Body.String(), "private-") {
				t.Errorf("HTTP error = %d %s, want %d without internal data", response.Code, response.Body, tt.code)
			}

			grpcStatus := status.Convert(err)
			if grpcStatus.Code() != tt.grpcCode || grpcStatus.Message() != tt.message {
				t.Errorf("gRPC error = %v, want %s/%s", grpcStatus, tt.grpcCode, tt.message)
			}

			if strings.Contains(grpcStatus.Proto().String(), "private-") {
				t.Errorf("gRPC error leaked private cause or metadata: %v", grpcStatus.Proto())
			}
		})
	}

	if err := Fallback(nil); err != nil {
		t.Errorf("Fallback(nil) = %v, want nil", err)
	}
}
