package validation

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateBackend(t *testing.T) {
	base := gateway.Backend{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "backend-1"},
		Spec: gateway.BackendSpec{
			Endpoints: []gateway.Endpoint{{Address: "api.example.com", Port: 8080, Weight: 1}},
		},
	}

	tests := []struct {
		name       string
		update     func(*gateway.Backend)
		wantFields []string
	}{
		{name: "valid"},
		{
			name:       "missing namespace",
			update:     func(b *gateway.Backend) { b.Namespace = "" },
			wantFields: []string{"metadata.namespace"},
		},
		{
			name:       "invalid namespace",
			update:     func(b *gateway.Backend) { b.Namespace = "Team.A" },
			wantFields: []string{"metadata.namespace"},
		},
		{
			name:       "empty endpoints",
			update:     func(u *gateway.Backend) { u.Spec.Endpoints = nil },
			wantFields: []string{"spec.endpoints"},
		},
		{
			name:       "invalid address",
			update:     func(u *gateway.Backend) { u.Spec.Endpoints[0].Address = "bad host" },
			wantFields: []string{"spec.endpoints[0].address"},
		},
		{
			name:       "invalid port and weight",
			update:     func(u *gateway.Backend) { u.Spec.Endpoints[0].Port = 0; u.Spec.Endpoints[0].Weight = 0 },
			wantFields: []string{"spec.endpoints[0].port", "spec.endpoints[0].weight"},
		},
		{
			name: "same address and different port",
			update: func(u *gateway.Backend) {
				u.Spec.Endpoints = append(u.Spec.Endpoints, gateway.Endpoint{
					Address: "api.example.com",
					Port:    8081,
					Weight:  1,
				})
			},
		},
		{
			name: "duplicate address and port",
			update: func(u *gateway.Backend) {
				u.Spec.Endpoints = append(u.Spec.Endpoints, gateway.Endpoint{
					Address: "api.example.com",
					Port:    8080,
					Weight:  2,
				})
			},
			wantFields: []string{"spec.endpoints[1]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := *base.DeepCopy()
			if tt.update != nil {
				tt.update(&candidate)
			}

			errs := ValidateBackend(&candidate)
			var gotFields []string
			for _, err := range errs {
				gotFields = append(gotFields, err.Field)
			}
			if !slices.Equal(gotFields, tt.wantFields) {
				t.Errorf("ValidateBackend(%q) fields = %v, want %v", tt.name, gotFields, tt.wantFields)
			}
		})
	}
}
