package upstream

import (
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
			name: "type is required",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Type = ""
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "model requires OpenAI protocol",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.Protocol = resource.UpstreamProtocolHTTP
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "credential requires TLS",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.TLS = nil
				return spec
			}(),
			wantErr: true,
		},
		{
			name: "credentialless model allows plaintext transport",
			spec: func() resource.UpstreamSpec {
				spec := modelUpstreamSpec()
				spec.TLS = nil
				spec.CredentialRef = ""
				spec.Endpoints[0].Port = 80
				return spec
			}(),
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

func modelUpstreamSpec() resource.UpstreamSpec {
	return resource.UpstreamSpec{
		Type:          resource.UpstreamTypeModel,
		Protocol:      resource.UpstreamProtocolOpenAI,
		TLS:           &resource.UpstreamTLS{ServerName: "api.openai.com"},
		CredentialRef: "credential-1",
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
