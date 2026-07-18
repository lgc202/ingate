package upstream

import (
	"testing"

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
			name: "model rejects HTTP protocol",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Protocol = resource.UpstreamProtocolHTTP
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
			name: "application rejects OpenAI protocol",
			request: func() CreateUpstreamReq {
				request := modelUpstreamRequest(nil, nil)
				request.Type = resource.UpstreamTypeApplication
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

func modelUpstreamRequest(tls *UpstreamTLS, apiKey *APIKeyConfig) CreateUpstreamReq {
	return CreateUpstreamReq{
		APIKey: apiKey,
		UpstreamConfig: UpstreamConfig{
			Name:              "OpenAI",
			Type:              resource.UpstreamTypeModel,
			Protocol:          resource.UpstreamProtocolOpenAI,
			TLS:               tls,
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
