package upstream

import (
	"encoding/json"
	"strings"
	"testing"

	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestNewGetUpstreamRespDoesNotExposeAPIKey(t *testing.T) {
	result := &upstreamservice.UpstreamResult{Upstream: &resource.Upstream{
		Spec: resource.UpstreamSpec{
			Authentication: &resource.UpstreamAuthentication{
				APIKey: &resource.APIKeyAuthentication{Value: "secret-value"},
			},
		},
	}}

	encoded, err := json.Marshal(NewGetUpstreamResp(result))
	if err != nil {
		t.Fatalf("json.Marshal(NewGetUpstreamResp()) error = %v, want nil", err)
	}
	if strings.Contains(string(encoded), "secret-value") || strings.Contains(string(encoded), `"apiKey":`) {
		t.Errorf("NewGetUpstreamResp() JSON = %s, want API key omitted", encoded)
	}
	if !strings.Contains(string(encoded), `"apiKeyConfigured":true`) {
		t.Errorf("NewGetUpstreamResp() JSON = %s, want apiKeyConfigured=true", encoded)
	}
}
