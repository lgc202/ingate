package upstreamcredential

import (
	"testing"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestCreateUpstreamCredentialReqValidate(t *testing.T) {
	tests := []struct {
		name    string
		request CreateUpstreamCredentialReq
		wantErr bool
	}{
		{
			name: "valid API key",
			request: CreateUpstreamCredentialReq{UpstreamCredentialConfig: UpstreamCredentialConfig{
				Name:   " OpenAI production ",
				Type:   resource.UpstreamCredentialTypeAPIKey,
				APIKey: &APIKeyConfig{Value: "secret-value"},
			}},
		},
		{
			name: "missing name",
			request: CreateUpstreamCredentialReq{UpstreamCredentialConfig: UpstreamCredentialConfig{
				Type:   resource.UpstreamCredentialTypeAPIKey,
				APIKey: &APIKeyConfig{Value: "secret-value"},
			}},
			wantErr: true,
		},
		{
			name: "API key contains space",
			request: CreateUpstreamCredentialReq{UpstreamCredentialConfig: UpstreamCredentialConfig{
				Name:   "OpenAI production",
				Type:   resource.UpstreamCredentialTypeAPIKey,
				APIKey: &APIKeyConfig{Value: "secret value"},
			}},
			wantErr: true,
		},
		{
			name: "API key contains Unicode separator",
			request: CreateUpstreamCredentialReq{UpstreamCredentialConfig: UpstreamCredentialConfig{
				Name:   "OpenAI production",
				Type:   resource.UpstreamCredentialTypeAPIKey,
				APIKey: &APIKeyConfig{Value: "secret\u2028value"},
			}},
			wantErr: true,
		},
		{
			name: "unsupported type",
			request: CreateUpstreamCredentialReq{UpstreamCredentialConfig: UpstreamCredentialConfig{
				Name:   "OpenAI production",
				Type:   resource.UpstreamCredentialType("OAuth2"),
				APIKey: &APIKeyConfig{Value: "secret-value"},
			}},
			wantErr: true,
		},
		{
			name: "missing API key",
			request: CreateUpstreamCredentialReq{UpstreamCredentialConfig: UpstreamCredentialConfig{
				Name: "OpenAI production",
				Type: resource.UpstreamCredentialTypeAPIKey,
			}},
			wantErr: true,
		},
		{
			name: "API key contains newline",
			request: CreateUpstreamCredentialReq{UpstreamCredentialConfig: UpstreamCredentialConfig{
				Name:   "OpenAI production",
				Type:   resource.UpstreamCredentialTypeAPIKey,
				APIKey: &APIKeyConfig{Value: "secret\r\ninjected"},
			}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("CreateUpstreamCredentialReq.Validate(%q) error = %v, want error presence = %t", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestUpdateUpstreamCredentialReqValidateAllowsOmittedAPIKey(t *testing.T) {
	request := UpdateUpstreamCredentialReq{
		Version: "1",
		UpstreamCredentialConfig: UpstreamCredentialConfig{
			Name: "OpenAI production",
			Type: resource.UpstreamCredentialTypeAPIKey,
		},
	}
	if err := request.Validate(); err != nil {
		t.Errorf("UpdateUpstreamCredentialReq.Validate() error = %v, want nil", err)
	}
}

func TestIDReqValidate(t *testing.T) {
	request := IDReq{ID: " credential-1 "}
	if err := request.Validate(); err != nil {
		t.Fatalf("IDReq.Validate() error = %v, want nil", err)
	}
	if request.ID != "credential-1" {
		t.Errorf("IDReq.Validate() ID = %q, want %q", request.ID, "credential-1")
	}
}
