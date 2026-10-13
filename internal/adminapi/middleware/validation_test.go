package middleware

import (
	"context"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

func TestValidateRequestsRejectsInvalidGateway(t *testing.T) {
	called := false
	next := kratosmiddleware.Handler(func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})

	_, err := ValidateRequests()(next)(context.Background(), &adminv1.CreateGatewayRequest{Namespace: "team-a"})
	if code := kratoserrors.Code(err); code != 400 {
		t.Errorf("ValidateRequests(empty CreateGatewayRequest) code = %d, want 400; error = %v", code, err)
	}
	if called {
		t.Error("ValidateRequests(empty CreateGatewayRequest) called next, want rejection")
	}
}

func TestValidateRequestsAllowsValidGateway(t *testing.T) {
	called := false
	next := kratosmiddleware.Handler(func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	request := &adminv1.CreateGatewayRequest{
		Namespace:   "team-a",
		DisplayName: "Public Gateway",
		Listeners: []*adminv1.Listener{{
			Name:     "http",
			Protocol: adminv1.ListenerProtocol_LISTENER_PROTOCOL_HTTP,
			Port:     8080,
		}},
	}

	_, err := ValidateRequests()(next)(context.Background(), request)
	if err != nil {
		t.Errorf("ValidateRequests(valid CreateGatewayRequest) error = %v, want nil", err)
	}
	if !called {
		t.Error("ValidateRequests(valid CreateGatewayRequest) did not call next")
	}
}
