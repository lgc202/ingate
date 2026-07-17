package gateway

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateGatewayRejectsNonHTTPProtocol(t *testing.T) {
	gateway := &resource.Gateway{
		Spec: resource.GatewaySpec{
			DisplayName: "测试网关",
			Listeners: []resource.Listener{
				{Name: "public", Protocol: resource.Protocol("HTTPS"), Port: 8443},
			},
			HostBindings: []resource.HostBinding{
				{ListenerRefs: []string{"public"}},
			},
		},
	}

	errs := validateGateway(gateway)
	if len(errs) != 1 {
		t.Fatalf("validateGateway(HTTPS listener) returned %d errors, want 1: %v", len(errs), errs)
	}
	if errs[0].Type != field.ErrorTypeNotSupported {
		t.Errorf("validateGateway(HTTPS listener) error type = %q, want %q", errs[0].Type, field.ErrorTypeNotSupported)
	}
	if got, want := errs[0].Field, "spec.listeners[0].protocol"; got != want {
		t.Errorf("validateGateway(HTTPS listener) error field = %q, want %q", got, want)
	}
}
