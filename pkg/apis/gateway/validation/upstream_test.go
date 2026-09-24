package validation

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateUpstream(t *testing.T) {
	base := gateway.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-1"},
		Spec: gateway.UpstreamSpec{
			Endpoints: []gateway.Endpoint{{Address: "api.example.com", Port: 8080, Weight: 1}},
		},
	}

	tests := []struct {
		name       string
		update     func(*gateway.Upstream)
		wantFields []string
	}{
		{name: "valid"},
		{
			name:       "empty endpoints",
			update:     func(u *gateway.Upstream) { u.Spec.Endpoints = nil },
			wantFields: []string{"spec.endpoints"},
		},
		{
			name:       "invalid address",
			update:     func(u *gateway.Upstream) { u.Spec.Endpoints[0].Address = "bad host" },
			wantFields: []string{"spec.endpoints[0].address"},
		},
		{
			name:       "invalid port and weight",
			update:     func(u *gateway.Upstream) { u.Spec.Endpoints[0].Port = 0; u.Spec.Endpoints[0].Weight = 0 },
			wantFields: []string{"spec.endpoints[0].port", "spec.endpoints[0].weight"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := *base.DeepCopy()
			if tt.update != nil {
				tt.update(&candidate)
			}

			errs := ValidateUpstream(&candidate)
			var gotFields []string
			for _, err := range errs {
				gotFields = append(gotFields, err.Field)
			}
			if !slices.Equal(gotFields, tt.wantFields) {
				t.Errorf("ValidateUpstream(%q) fields = %v, want %v", tt.name, gotFields, tt.wantFields)
			}
		})
	}
}
