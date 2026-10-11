package validation

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateHTTPRoute(t *testing.T) {
	base := gateway.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "route-1"},
		Spec: gateway.HTTPRouteSpec{
			ParentRefs: []gateway.ParentReference{{Name: "gateway-1", SectionName: "http"}},
			Rules: []gateway.HTTPRouteRule{{
				Matches:     []gateway.HTTPRouteMatch{{Path: &gateway.PathMatch{Type: gateway.PathMatchPrefix, Value: "/api"}}},
				BackendRefs: []gateway.BackendReference{{Name: "backend-1"}},
			}},
		},
	}

	tests := []struct {
		name       string
		update     func(*gateway.HTTPRoute)
		wantFields []string
	}{
		{name: "valid"},
		{name: "default match", update: func(r *gateway.HTTPRoute) { r.Spec.Rules[0].Matches = nil }},
		{name: "missing parents", update: func(r *gateway.HTTPRoute) { r.Spec.ParentRefs = nil }, wantFields: []string{"spec.parentRefs"}},
		{
			name: "duplicate parent",
			update: func(r *gateway.HTTPRoute) {
				r.Spec.ParentRefs = append(r.Spec.ParentRefs, r.Spec.ParentRefs[0])
			},
			wantFields: []string{"spec.parentRefs[1]"},
		},
		{
			name: "overlapping parents",
			update: func(r *gateway.HTTPRoute) {
				r.Spec.ParentRefs = append(r.Spec.ParentRefs, gateway.ParentReference{Name: "gateway-1"})
			},
			wantFields: []string{"spec.parentRefs[0]"},
		},
		{
			name: "distinct sections",
			update: func(r *gateway.HTTPRoute) {
				r.Spec.ParentRefs = append(r.Spec.ParentRefs, gateway.ParentReference{Name: "gateway-1", SectionName: "admin"})
			},
		},
		{name: "missing rules", update: func(r *gateway.HTTPRoute) { r.Spec.Rules = nil }, wantFields: []string{"spec.rules"}},
		{
			name: "two backends",
			update: func(r *gateway.HTTPRoute) {
				r.Spec.Rules[0].BackendRefs = append(r.Spec.Rules[0].BackendRefs, gateway.BackendReference{Name: "other"})
			},
			wantFields: []string{"spec.rules[0].backendRefs"},
		},
		{
			name: "duplicate rule names",
			update: func(r *gateway.HTTPRoute) {
				r.Spec.Rules[0].Name = "api"
				r.Spec.Rules = append(r.Spec.Rules, *r.Spec.Rules[0].DeepCopy())
			},
			wantFields: []string{"spec.rules[1].name"},
		},
		{
			name:       "missing gateway",
			update:     func(r *gateway.HTTPRoute) { r.Spec.ParentRefs[0].Name = "" },
			wantFields: []string{"spec.parentRefs[0].name"},
		},
		{
			name:       "invalid listener name",
			update:     func(r *gateway.HTTPRoute) { r.Spec.ParentRefs[0].SectionName = "api.public" },
			wantFields: []string{"spec.parentRefs[0].sectionName"},
		},
		{
			name:       "unsupported path type",
			update:     func(r *gateway.HTTPRoute) { r.Spec.Rules[0].Matches[0].Path.Type = "Regex" },
			wantFields: []string{"spec.rules[0].matches[0].path.type"},
		},
		{
			name:       "path contains query",
			update:     func(r *gateway.HTTPRoute) { r.Spec.Rules[0].Matches[0].Path.Value = "/api?q=1" },
			wantFields: []string{"spec.rules[0].matches[0].path.value"},
		},
		{
			name:       "path contains consecutive slashes",
			update:     func(r *gateway.HTTPRoute) { r.Spec.Rules[0].Matches[0].Path.Value = "/api//v1" },
			wantFields: []string{"spec.rules[0].matches[0].path.value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := *base.DeepCopy()
			if tt.update != nil {
				tt.update(&candidate)
			}

			errs := ValidateHTTPRoute(&candidate)
			var gotFields []string
			for _, err := range errs {
				gotFields = append(gotFields, err.Field)
			}
			if !slices.Equal(gotFields, tt.wantFields) {
				t.Errorf("ValidateHTTPRoute(%q) fields = %v, want %v", tt.name, gotFields, tt.wantFields)
			}
		})
	}
}
