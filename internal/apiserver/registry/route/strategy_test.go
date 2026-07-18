package route

import (
	"testing"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateRouteModelRouting(t *testing.T) {
	tests := []struct {
		name    string
		rule    resource.RouteRule
		wantErr bool
	}{
		{
			name: "model route",
			rule: modelRouteRule(),
		},
		{
			name: "rejects ordinary upstream refs",
			rule: func() resource.RouteRule {
				rule := modelRouteRule()
				rule.UpstreamRefs = []resource.UpstreamRef{{Name: "model-1", Weight: 100}}
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "requires POST only",
			rule: func() resource.RouteRule {
				rule := modelRouteRule()
				rule.Methods = []string{"GET"}
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "requires Chat Completions path",
			rule: func() resource.RouteRule {
				rule := modelRouteRule()
				rule.PathPrefix = "/chat"
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "requires upstream ref",
			rule: func() resource.RouteRule {
				rule := modelRouteRule()
				rule.ModelRouting.UpstreamRef = ""
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "rejects whitespace around upstream ref",
			rule: func() resource.RouteRule {
				rule := modelRouteRule()
				rule.ModelRouting.UpstreamRef = " model-1 "
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "requires model mappings",
			rule: func() resource.RouteRule {
				rule := modelRouteRule()
				rule.ModelRouting.Models = nil
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "rejects duplicate client model",
			rule: func() resource.RouteRule {
				rule := modelRouteRule()
				rule.ModelRouting.Models = append(rule.ModelRouting.Models, rule.ModelRouting.Models[0])
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "rejects whitespace around model names",
			rule: func() resource.RouteRule {
				rule := modelRouteRule()
				rule.ModelRouting.Models[0].UpstreamModel = " gpt-4o-mini "
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "rejects retry",
			rule: func() resource.RouteRule {
				rule := modelRouteRule()
				rule.Retry = &resource.RouteRetry{Attempts: 1, PerTryTimeoutMillis: 1000}
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "rejects managed authorization header",
			rule: func() resource.RouteRule {
				rule := modelRouteRule()
				rule.Filters = []resource.RouteFilter{
					{
						Type: resource.RouteFilterRequestHeaderModifier,
						RequestHeaderModifier: &resource.HeaderModifier{
							Set: []resource.HeaderValue{{Name: "Authorization", Value: "Bearer manual-secret"}},
						},
					},
				}
				return rule
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &resource.Route{Spec: resource.RouteSpec{
				ParentRefs: []resource.ParentRef{{Name: "gateway-1"}},
				Rules:      []resource.RouteRule{tt.rule},
			}}
			errs := validateRoute(route)
			if gotErr := len(errs) > 0; gotErr != tt.wantErr {
				t.Errorf("validateRoute() errors = %v, want error presence = %t", errs, tt.wantErr)
			}
		})
	}
}

func modelRouteRule() resource.RouteRule {
	return resource.RouteRule{
		Name:       "chat",
		PathPrefix: "/v1/chat/completions",
		Methods:    []string{"POST"},
		ModelRouting: &resource.ModelRouting{
			UpstreamRef: "model-1",
			Models: []resource.ModelRoute{
				{Model: "chat-default", UpstreamModel: "gpt-4o-mini"},
			},
		},
	}
}
