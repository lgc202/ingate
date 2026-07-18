package route

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRouteRuleValidateModelRouting(t *testing.T) {
	tests := []struct {
		name    string
		rule    RouteRule
		wantErr bool
	}{
		{
			name: "model route",
			rule: modelRouteRule(),
		},
		{
			name: "rejects ordinary targets",
			rule: func() RouteRule {
				rule := modelRouteRule()
				rule.Targets = []RouteTarget{{UpstreamID: "model-1", Weight: 100}}
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "requires POST only",
			rule: func() RouteRule {
				rule := modelRouteRule()
				rule.Methods = []string{"GET"}
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "requires Chat Completions path",
			rule: func() RouteRule {
				rule := modelRouteRule()
				rule.PathPrefix = "/chat"
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "requires model upstream",
			rule: func() RouteRule {
				rule := modelRouteRule()
				rule.ModelRouting.UpstreamID = ""
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "requires model mappings",
			rule: func() RouteRule {
				rule := modelRouteRule()
				rule.ModelRouting.Models = nil
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "requires client model name",
			rule: func() RouteRule {
				rule := modelRouteRule()
				rule.ModelRouting.Models[0].Model = ""
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "rejects retry",
			rule: func() RouteRule {
				rule := modelRouteRule()
				rule.Retry = &RouteRetryReq{Attempts: 1, PerTryTimeoutMillis: 1000}
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "rejects duplicate model",
			rule: func() RouteRule {
				rule := modelRouteRule()
				rule.ModelRouting.Models = append(rule.ModelRouting.Models, rule.ModelRouting.Models[0])
				return rule
			}(),
			wantErr: true,
		},
		{
			name: "rejects managed authorization header",
			rule: func() RouteRule {
				rule := modelRouteRule()
				rule.RequestHeaderModifier = &HeaderModifierReq{
					Set: []HeaderValueReq{{Name: "Authorization", Value: "Bearer manual-secret"}},
				}
				return rule
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("RouteRule.Validate() error = %v, want error presence = %t", err, tt.wantErr)
			}
		})
	}
}

func TestRouteRuleValidateNormalizesModelRouting(t *testing.T) {
	rule := modelRouteRule()
	rule.ModelRouting.UpstreamID = " model-1 "
	rule.ModelRouting.Models[0].Model = " chat-default "
	rule.ModelRouting.Models[0].UpstreamModel = " gpt-4o-mini "

	if err := rule.Validate(); err != nil {
		t.Fatalf("RouteRule.Validate() error = %v, want nil", err)
	}
	want := &ModelRouting{
		UpstreamID: "model-1",
		Models: []ModelRoute{
			{Model: "chat-default", UpstreamModel: "gpt-4o-mini"},
		},
	}
	if diff := cmp.Diff(want, rule.ModelRouting); diff != "" {
		t.Errorf("RouteRule.Validate() modelRouting mismatch (-want +got):\n%s", diff)
	}
}

func modelRouteRule() RouteRule {
	return RouteRule{
		Name:       "chat",
		PathPrefix: "/v1/chat/completions",
		Methods:    []string{"POST"},
		ModelRouting: &ModelRouting{
			UpstreamID: "model-1",
			Models: []ModelRoute{
				{
					Model:         "chat-default",
					UpstreamModel: "gpt-4o-mini",
				},
			},
		},
	}
}
