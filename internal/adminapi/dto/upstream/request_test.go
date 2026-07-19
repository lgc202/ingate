package upstream

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestCreateUpstreamReqValidateModelAuthentication(t *testing.T) {
	tests := []struct {
		name    string
		request CreateUpstreamReq
		wantErr bool
	}{
		{
			name:    "OpenAI model with API key over HTTPS",
			request: modelUpstreamRequest(&UpstreamTLS{ServerName: "api.openai.com"}, &APIKeyConfig{Value: "sk-test-secret"}),
		},
		{
			name: "Anthropic accepts general HTTP header API key",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(&UpstreamTLS{ServerName: "api.anthropic.com"}, &APIKeyConfig{Value: "secret value:1"})
				request.Protocol = resource.UpstreamProtocolAnthropic
				request.Model.Provider = resource.ModelProviderAnthropic
				return request
			}(),
		},
		{
			name: "model requires config",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Model = nil
				return request
			}(),
			wantErr: true,
		},
		{
			name:    "API key requires HTTPS",
			request: modelUpstreamRequest(nil, &APIKeyConfig{Value: "sk-test-secret"}),
			wantErr: true,
		},
		{
			name: "model without API key allows HTTP transport",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Endpoints[0].Port = 80
				return request
			}(),
		},
		{
			name:    "API key rejects unsafe value",
			request: modelUpstreamRequest(&UpstreamTLS{ServerName: "api.openai.com"}, &APIKeyConfig{Value: "secret\r\ninjected"}),
			wantErr: true,
		},
		{
			name:    "API key rejects tab",
			request: modelUpstreamRequest(&UpstreamTLS{ServerName: "api.openai.com"}, &APIKeyConfig{Value: "secret\tvalue"}),
			wantErr: true,
		},
		{
			name: "application rejects OpenAI protocol",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Type = resource.UpstreamTypeApplication
				request.Model = nil
				return request
			}(),
			wantErr: true,
		},
		{
			name: "provider rejects mismatched protocol",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Model.Provider = resource.ModelProviderGemini
				return request
			}(),
			wantErr: true,
		},
		{
			name: "API base path rejects query",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Model.APIBasePath = "/v1?debug=true"
				return request
			}(),
			wantErr: true,
		},
		{
			name: "model catalog requires enabled item",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Model.Models[0].Enabled = false
				return request
			}(),
			wantErr: true,
		},
		{
			name: "model catalog rejects duplicate names",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Model.Models = append(request.Model.Models, request.Model.Models[0])
				return request
			}(),
			wantErr: true,
		},
		{
			name: "model catalog requires display name",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Model.Models[0].DisplayName = ""
				return request
			}(),
			wantErr: true,
		},
		{
			name:    "invalid HTTPS server name",
			request: modelUpstreamRequest(&UpstreamTLS{ServerName: "https://api.openai.com"}, nil),
			wantErr: true,
		},
		{
			name: "duplicate endpoint ID",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Endpoints = append(request.Endpoints, request.Endpoints[0])
				return request
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("CreateUpstreamReq.Validate() error = %v, want error presence = %t", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateUpstreamReqValidateRejectsConflictingAPIKeyOperations(t *testing.T) {
	request := UpdateUpstreamReq{
		Version:      "1",
		APIKey:       &APIKeyConfig{Value: "sk-test-secret"},
		RemoveAPIKey: true,
		UpstreamConfig: modelUpstreamRequest(
			&UpstreamTLS{ServerName: "api.openai.com"},
			nil,
		).UpstreamConfig,
	}

	if err := request.Validate(); err == nil {
		t.Fatal("UpdateUpstreamReq.Validate(conflicting API key operations) error = nil, want non-nil")
	}
}

func TestCreateUpstreamReqParamsIncludesModelConfig(t *testing.T) {
	request := modelUpstreamRequest(nil, nil)
	if err := request.Validate(); err != nil {
		t.Fatalf("CreateUpstreamReq.Validate() error = %v, want nil", err)
	}
	want := &upstreamservice.ModelParams{
		Provider:    resource.ModelProviderOpenAI,
		APIBasePath: "/v1",
		Models: []upstreamservice.ModelCatalogItemParams{
			{Name: "gpt-4o-mini", DisplayName: "GPT-4o mini", Enabled: true},
		},
	}

	got := request.Params().Model
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("CreateUpstreamReq.Params() model mismatch (-want +got):\n%s", diff)
	}
}

func modelUpstreamRequest(tls *UpstreamTLS, apiKey *APIKeyConfig) CreateUpstreamReq {
	return CreateUpstreamReq{
		APIKey: apiKey,
		UpstreamConfig: UpstreamConfig{
			Name:     "OpenAI",
			Type:     resource.UpstreamTypeModel,
			Protocol: resource.UpstreamProtocolOpenAI,
			TLS:      tls,
			Model: &ModelConfig{
				Provider:    resource.ModelProviderOpenAI,
				APIBasePath: "/v1",
				Models: []ModelCatalogItem{
					{Name: "gpt-4o-mini", DisplayName: "GPT-4o mini", Enabled: true},
				},
			},
			LoadBalancePolicy: resource.UpstreamLoadBalancePolicyRoundRobin,
			Endpoints: []UpstreamEndpoint{
				{
					ID:      "primary",
					Address: "api.openai.com",
					Port:    443,
					Weight:  100,
					Enabled: true,
				},
			},
		},
	}
}
