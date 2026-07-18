package upstream

import (
	"testing"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestUpstreamConfigValidateModelProtocol(t *testing.T) {
	tests := []struct {
		name    string
		config  UpstreamConfig
		wantErr bool
	}{
		{
			name: "OpenAI model with HTTPS",
			config: modelUpstreamConfig(&UpstreamTLS{
				ServerName: "api.openai.com",
			}),
		},
		{
			name: "model rejects HTTP protocol",
			config: func() UpstreamConfig {
				config := modelUpstreamConfig(nil)
				config.Protocol = resource.UpstreamProtocolHTTP
				return config
			}(),
			wantErr: true,
		},
		{
			name:    "credential requires HTTPS",
			config:  modelUpstreamConfig(nil),
			wantErr: true,
		},
		{
			name: "credentialless model allows HTTP transport",
			config: func() UpstreamConfig {
				config := modelUpstreamConfig(nil)
				config.CredentialID = ""
				config.Endpoints[0].Port = 80
				return config
			}(),
		},
		{
			name: "application rejects OpenAI protocol",
			config: func() UpstreamConfig {
				config := modelUpstreamConfig(nil)
				config.Type = resource.UpstreamTypeApplication
				return config
			}(),
			wantErr: true,
		},
		{
			name: "invalid HTTPS server name",
			config: modelUpstreamConfig(&UpstreamTLS{
				ServerName: "https://api.openai.com",
			}),
			wantErr: true,
		},
		{
			name: "duplicate endpoint ID",
			config: func() UpstreamConfig {
				config := modelUpstreamConfig(nil)
				config.Endpoints = append(config.Endpoints, config.Endpoints[0])
				return config
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("UpstreamConfig.Validate() error = %v, want error presence = %t", err, tt.wantErr)
			}
		})
	}
}

func modelUpstreamConfig(tls *UpstreamTLS) UpstreamConfig {
	return UpstreamConfig{
		Name:              "OpenAI",
		Type:              resource.UpstreamTypeModel,
		Protocol:          resource.UpstreamProtocolOpenAI,
		TLS:               tls,
		CredentialID:      "credential-1",
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
	}
}
