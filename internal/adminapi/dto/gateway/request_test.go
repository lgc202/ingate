package gateway

import (
	"slices"
	"testing"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestGatewayConfigValidateListeners(t *testing.T) {
	tests := []struct {
		name      string
		listeners []GatewayListener
		wantErr   bool
	}{
		{
			name:      "http",
			listeners: []GatewayListener{{Protocol: resource.ProtocolHTTP, Port: 8080}},
		},
		{
			name:      "https",
			listeners: []GatewayListener{{Protocol: resource.ProtocolHTTPS, Port: 8443, CertificateID: "certificate-1"}},
		},
		{
			name: "http and https",
			listeners: []GatewayListener{
				{Protocol: resource.ProtocolHTTPS, Port: 8443, CertificateID: "certificate-1"},
				{Protocol: resource.ProtocolHTTP, Port: 8080},
			},
		},
		{
			name:    "missing listener",
			wantErr: true,
		},
		{
			name:      "wrong http port",
			listeners: []GatewayListener{{Protocol: resource.ProtocolHTTP, Port: 80}},
			wantErr:   true,
		},
		{
			name:      "https without certificate",
			listeners: []GatewayListener{{Protocol: resource.ProtocolHTTPS, Port: 8443}},
			wantErr:   true,
		},
		{
			name:      "http with certificate",
			listeners: []GatewayListener{{Protocol: resource.ProtocolHTTP, Port: 8080, CertificateID: "certificate-1"}},
			wantErr:   true,
		},
		{
			name: "duplicate protocol",
			listeners: []GatewayListener{
				{Protocol: resource.ProtocolHTTP, Port: 8080},
				{Protocol: resource.ProtocolHTTP, Port: 8080},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GatewayConfig{Name: "gateway", Listeners: tt.listeners}
			err := config.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("GatewayConfig.Validate(%v) error = %v, want error presence = %t", tt.listeners, err, tt.wantErr)
			}
		})
	}
}

func TestGatewayConfigValidateHostnames(t *testing.T) {
	tests := []struct {
		name      string
		hostnames []string
		want      []string
		wantErr   bool
	}{
		{
			name:      "normalizes and removes exact duplicates",
			hostnames: []string{" API.Example.COM ", "api.example.com"},
			want:      []string{"api.example.com"},
		},
		{
			name:      "wildcard overlaps exact hostname",
			hostnames: []string{"*.example.com", "api.example.com"},
			wantErr:   true,
		},
		{
			name:      "nested wildcards overlap",
			hostnames: []string{"*.example.com", "*.api.example.com"},
			wantErr:   true,
		},
		{
			name:      "wildcard and apex do not overlap",
			hostnames: []string{"*.example.com", "example.com"},
			want:      []string{"*.example.com", "example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GatewayConfig{
				Name:      "gateway",
				Listeners: []GatewayListener{{Protocol: resource.ProtocolHTTP, Port: 8080}},
				Hostnames: tt.hostnames,
			}

			err := config.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("GatewayConfig.Validate(%v) error = %v, want error presence = %t", tt.hostnames, err, tt.wantErr)
			}
			if !tt.wantErr && !slices.Equal(config.Hostnames, tt.want) {
				t.Errorf("GatewayConfig.Validate(%v) hostnames = %v, want %v", tt.hostnames, config.Hostnames, tt.want)
			}
		})
	}
}
