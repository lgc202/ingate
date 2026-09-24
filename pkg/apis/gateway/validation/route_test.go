package validation

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateRoute(t *testing.T) {
	base := gateway.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route-1"},
		Spec: gateway.RouteSpec{
			GatewayName:  "gateway-1",
			ListenerName: "http",
			Path:         gateway.PathMatch{Type: gateway.PathMatchPrefix, Value: "/api"},
			UpstreamName: "backend-1",
		},
	}

	tests := []struct {
		name       string
		update     func(*gateway.Route)
		wantFields []string
	}{
		{name: "valid"},
		{
			name:       "missing gateway",
			update:     func(r *gateway.Route) { r.Spec.GatewayName = "" },
			wantFields: []string{"spec.gatewayName"},
		},
		{
			name:       "invalid listener name",
			update:     func(r *gateway.Route) { r.Spec.ListenerName = "api.public" },
			wantFields: []string{"spec.listenerName"},
		},
		{
			name:       "unsupported path type",
			update:     func(r *gateway.Route) { r.Spec.Path.Type = "Regex" },
			wantFields: []string{"spec.path.type"},
		},
		{
			name:       "path contains query",
			update:     func(r *gateway.Route) { r.Spec.Path.Value = "/api?q=1" },
			wantFields: []string{"spec.path.value"},
		},
		{
			name:       "path contains consecutive slashes",
			update:     func(r *gateway.Route) { r.Spec.Path.Value = "/api//v1" },
			wantFields: []string{"spec.path.value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := *base.DeepCopy()
			if tt.update != nil {
				tt.update(&candidate)
			}

			errs := ValidateRoute(&candidate)
			var gotFields []string
			for _, err := range errs {
				gotFields = append(gotFields, err.Field)
			}
			if !slices.Equal(gotFields, tt.wantFields) {
				t.Errorf("ValidateRoute(%q) fields = %v, want %v", tt.name, gotFields, tt.wantFields)
			}
		})
	}
}
