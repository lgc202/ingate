package upstreamcredential

import (
	"encoding/json"
	"strings"
	"testing"

	credentialservice "github.com/lgc202/ingate/internal/adminapi/service/upstreamcredential"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestNewGetUpstreamCredentialRespDoesNotExposeAPIKey(t *testing.T) {
	result := &credentialservice.CredentialResult{Credential: &resource.UpstreamCredential{
		Spec: resource.UpstreamCredentialSpec{
			DisplayName: "OpenAI production",
			Type:        resource.UpstreamCredentialTypeAPIKey,
			APIKey:      &resource.APIKeyCredential{Value: "secret-value"},
		},
	}}

	encoded, err := json.Marshal(NewGetUpstreamCredentialResp(result))
	if err != nil {
		t.Fatalf("json.Marshal(NewGetUpstreamCredentialResp()) error = %v, want nil", err)
	}
	if strings.Contains(string(encoded), "secret-value") || strings.Contains(string(encoded), "apiKey") {
		t.Error("NewGetUpstreamCredentialResp() exposed API key data")
	}
	if !strings.Contains(string(encoded), `"configured":true`) {
		t.Errorf("NewGetUpstreamCredentialResp() JSON = %s, want configured=true", encoded)
	}
}
