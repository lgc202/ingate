package validation

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateGateway(t *testing.T) {
	base := gateway.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"},
		Spec: gateway.GatewaySpec{
			Listeners: []gateway.Listener{{Name: "http", Protocol: gateway.ProtocolHTTP, Port: 8080}},
		},
	}

	tests := []struct {
		name       string
		update     func(*gateway.Gateway)
		wantFields []string
	}{
		{name: "valid", wantFields: nil},
		{
			name:       "empty listeners",
			update:     func(g *gateway.Gateway) { g.Spec.Listeners = nil },
			wantFields: []string{"spec.listeners"},
		},
		{
			name:       "duplicate name",
			update:     func(g *gateway.Gateway) { g.Spec.Listeners = append(g.Spec.Listeners, g.Spec.Listeners[0]) },
			wantFields: []string{"spec.listeners[1].name"},
		},
		{
			name: "overlapping wildcard host",
			update: func(g *gateway.Gateway) {
				g.Spec.Listeners = append(g.Spec.Listeners, gateway.Listener{
					Name: "other", Protocol: gateway.ProtocolHTTP, Port: 8080, Hostname: "api.example.com",
				})
			},
			wantFields: []string{"spec.listeners[1]"},
		},
		{
			name: "distinct hosts on same port",
			update: func(g *gateway.Gateway) {
				g.Spec.Listeners[0].Hostname = "one.example.com"
				g.Spec.Listeners = append(g.Spec.Listeners, gateway.Listener{
					Name: "other", Protocol: gateway.ProtocolHTTP, Port: 8080, Hostname: "two.example.com",
				})
			},
		},
		{
			name:       "unsupported protocol",
			update:     func(g *gateway.Gateway) { g.Spec.Listeners[0].Protocol = "HTTPS" },
			wantFields: []string{"spec.listeners[0].protocol"},
		},
		{
			name:       "invalid port",
			update:     func(g *gateway.Gateway) { g.Spec.Listeners[0].Port = 0 },
			wantFields: []string{"spec.listeners[0].port"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := *base.DeepCopy()
			if tt.update != nil {
				tt.update(&candidate)
			}

			errs := ValidateGateway(&candidate)
			var gotFields []string
			for _, err := range errs {
				gotFields = append(gotFields, err.Field)
			}
			if !slices.Equal(gotFields, tt.wantFields) {
				t.Errorf("ValidateGateway(%q) fields = %v, want %v", tt.name, gotFields, tt.wantFields)
			}
		})
	}
}
