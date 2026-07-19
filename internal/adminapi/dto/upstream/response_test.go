package upstream

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

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

func TestNewGetUpstreamRespIncludesModelConfig(t *testing.T) {
	result := &upstreamservice.UpstreamResult{Upstream: &resource.Upstream{
		Spec: resource.UpstreamSpec{
			Model: &resource.ModelSpec{
				Provider:    resource.ModelProviderAnthropic,
				APIBasePath: "/v1",
				Models: []resource.ModelCatalogItem{
					{Name: "claude-sonnet", DisplayName: "Claude Sonnet", Enabled: true},
				},
			},
		},
	}}
	want := &ModelConfig{
		Provider:    resource.ModelProviderAnthropic,
		APIBasePath: "/v1",
		Models: []ModelCatalogItem{
			{Name: "claude-sonnet", DisplayName: "Claude Sonnet", Enabled: true},
		},
	}

	got := NewGetUpstreamResp(result).Model
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("NewGetUpstreamResp() model mismatch (-want +got):\n%s", diff)
	}
}
