package config

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestCompilerRejectsNonHTTPGatewayListener(t *testing.T) {
	result := (Compiler{}).Compile(ResourceSet{
		Gateways: []*gatewayv1.Gateway{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"},
				Spec: gatewayv1.GatewaySpec{
					Enabled: true,
					Listeners: []gatewayv1.Listener{
						{Name: "public", Protocol: gatewayv1.Protocol("HTTPS"), Port: 8443},
					},
				},
			},
		},
	})

	if !result.HasErrors() {
		t.Fatal("Compiler.Compile(HTTPS listener) HasErrors() = false, want true")
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("Compiler.Compile(HTTPS listener) returned %d diagnostics, want 1: %v", len(result.Diagnostics), result.Diagnostics)
	}
	if got, want := result.Diagnostics[0].Reason, ReasonUnsupported; got != want {
		t.Errorf("Compiler.Compile(HTTPS listener) diagnostic reason = %q, want %q", got, want)
	}
}
