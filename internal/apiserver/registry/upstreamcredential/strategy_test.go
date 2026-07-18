package upstreamcredential

import (
	"testing"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateUpstreamCredential(t *testing.T) {
	tests := []struct {
		name       string
		credential *resource.UpstreamCredential
		wantFields []string
	}{
		{
			name: "valid API key",
			credential: &resource.UpstreamCredential{Spec: resource.UpstreamCredentialSpec{
				DisplayName: "OpenAI production",
				Type:        resource.UpstreamCredentialTypeAPIKey,
				APIKey:      &resource.APIKeyCredential{Value: "secret-value"},
			}},
		},
		{
			name:       "required display name and type",
			credential: &resource.UpstreamCredential{},
			wantFields: []string{"spec.displayName", "spec.type"},
		},
		{
			name: "blank display name",
			credential: &resource.UpstreamCredential{Spec: resource.UpstreamCredentialSpec{
				DisplayName: "   ",
				Type:        resource.UpstreamCredentialTypeAPIKey,
				APIKey:      &resource.APIKeyCredential{Value: "secret-value"},
			}},
			wantFields: []string{"spec.displayName"},
		},
		{
			name: "unsupported type",
			credential: &resource.UpstreamCredential{Spec: resource.UpstreamCredentialSpec{
				DisplayName: "unsupported",
				Type:        resource.UpstreamCredentialType("OAuth2"),
			}},
			wantFields: []string{"spec.type"},
		},
		{
			name: "missing API key",
			credential: &resource.UpstreamCredential{Spec: resource.UpstreamCredentialSpec{
				DisplayName: "OpenAI production",
				Type:        resource.UpstreamCredentialTypeAPIKey,
			}},
			wantFields: []string{"spec.apiKey"},
		},
		{
			name: "empty API key value",
			credential: &resource.UpstreamCredential{Spec: resource.UpstreamCredentialSpec{
				DisplayName: "OpenAI production",
				Type:        resource.UpstreamCredentialTypeAPIKey,
				APIKey:      &resource.APIKeyCredential{},
			}},
			wantFields: []string{"spec.apiKey.value"},
		},
		{
			name: "API key contains newline",
			credential: &resource.UpstreamCredential{Spec: resource.UpstreamCredentialSpec{
				DisplayName: "OpenAI production",
				Type:        resource.UpstreamCredentialTypeAPIKey,
				APIKey:      &resource.APIKeyCredential{Value: "secret\r\ninjected"},
			}},
			wantFields: []string{"spec.apiKey.value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateUpstreamCredential(tt.credential)
			if len(errs) != len(tt.wantFields) {
				t.Fatalf("validateUpstreamCredential(%q) errors = %v, want fields %v", tt.name, errs, tt.wantFields)
			}
			for i, wantField := range tt.wantFields {
				if errs[i].Field != wantField {
					t.Errorf("validateUpstreamCredential(%q) error[%d].Field = %q, want %q", tt.name, i, errs[i].Field, wantField)
				}
			}
		})
	}
}
