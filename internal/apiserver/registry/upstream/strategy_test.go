package upstream

import (
	"strings"
	"testing"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateUpstreamModelConnection(t *testing.T) {
	tests := []struct {
		name    string
		spec    resource.UpstreamSpec
		wantErr bool
	}{
		{
			name: "OpenAI model over TLS",
			spec: modelUpstreamSpec(),
		},
		{
			name: "Anthropic API key accepts HTTP header value",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Protocol = resource.UpstreamProtocolAnthropic
				spec.Model.Provider = resource.ModelProviderAnthropic
				spec.Authentication.APIKey.Value = "secret value:1"
				return spec
			}(),
		},
		{
			name: "type is required",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Type = ""
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "model requires model config",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Model = nil
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "provider must match protocol",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Model.Provider = resource.ModelProviderGemini
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "API base path must be normalized",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Model.APIBasePath = "/v1/"
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "model catalog requires enabled item",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Model.Models[0].Enabled = false
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "model catalog requires unique names",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Model.Models = append(spec.Model.Models, resource.ModelCatalogItem{
					Name:        spec.Model.Models[0].Name,
					DisplayName: "Duplicate",
					Enabled:     true,
				})
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "model catalog requires display name",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Model.Models[0].DisplayName = ""
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "non-model rejects model config",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Type = resource.UpstreamTypeApplication
				spec.Protocol = resource.UpstreamProtocolHTTP
				spec.Authentication = nil
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "API key requires TLS",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.TLS = nil
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "model without API key allows plaintext transport",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.TLS = nil
				spec.Authentication = nil
				spec.Endpoints[0].Port = 80
				return spec
			}(),
		},
		{
			name: "API key rejects unsafe value",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Authentication.APIKey.Value = "secret\r\ninjected"
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "API key rejects tab",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Authentication.APIKey.Value = "secret\tvalue"
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "TLS requires valid server name",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.TLS.ServerName = "https://api.openai.com"
				return spec
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateUpstream(&resource.Upstream{Spec: tt.spec})
			if gotErr := len(errs) > 0; gotErr != tt.wantErr {
				t.Errorf("validateUpstream() errors = %v, want error presence = %t", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateUpstreamDoesNotExposeAPIKey(t *testing.T) {
	spec := modelUpstreamSpec()
	spec.Authentication.APIKey.Value = "secret\r\ninjected"

	errs := validateUpstream(&resource.Upstream{Spec: spec})
	if len(errs) == 0 {
		t.Fatal("validateUpstream(unsafe API key) errors = nil, want non-empty")
	}
	if strings.Contains(errs.ToAggregate().Error(), spec.Authentication.APIKey.Value) {
		t.Error("validateUpstream(unsafe API key) exposed the API key in its error")
	}
}

func modelUpstreamSpec() resource.UpstreamSpec {
	return resource.UpstreamSpec{
		Type:     resource.UpstreamTypeModel,
		Protocol: resource.UpstreamProtocolOpenAI,
		Model: &resource.ModelSpec{
			Provider:    resource.ModelProviderOpenAI,
			APIBasePath: "/v1",
			Models: []resource.ModelCatalogItem{
				{Name: "gpt-4o-mini", DisplayName: "GPT-4o mini", Enabled: true},
			},
		},
		TLS: &resource.UpstreamTLS{ServerName: "api.openai.com"},
		Authentication: &resource.UpstreamAuthentication{
			APIKey: &resource.APIKeyAuthentication{Value: "sk-test-secret"},
		},
		Endpoints: []resource.Endpoint{
			{
				Name:    "primary",
				Address: "api.openai.com",
				Port:    443,
				Weight:  100,
				Enabled: true,
			},
		},
	}
}
