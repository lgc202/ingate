package acl

import "testing"

func TestParsePluginConfigRejectsUnknownFieldsAndMultipleValues(t *testing.T) {
	cfg, err := ParsePluginConfig([]byte(`{"routes":[]}`))
	if err != nil {
		t.Fatalf("ParsePluginConfig() error = %v", err)
	}
	if len(cfg.Routes) != 0 {
		t.Fatalf("len(Routes) = %d, want 0", len(cfg.Routes))
	}

	for _, data := range []string{
		`{"schemaVersion":"v1","routes":[]}`,
		`{"routes":[{"gatewayName":"gw","routeName":"route","ruleName":"primary","bindings":[]}]}`,
		`{"routes":[{"gatewayName":"gw","routeName":"route","bindings":[{"name":"binding","target":{"kind":"Route","name":"route"},"policies":[{"name":"policy","displayName":"Policy","rules":[]}]}]}]}`,
		`{"routes":[]} {}`,
	} {
		if _, err := ParsePluginConfig([]byte(data)); err == nil {
			t.Errorf("ParsePluginConfig(%s) error = nil, want rejection", data)
		}
	}
}

func TestRouteResponseDefaults(t *testing.T) {
	var policy Policy
	if policy.DeniedStatusCode() != defaultDeniedStatusCode {
		t.Fatalf("DeniedStatusCode() = %d, want default", policy.DeniedStatusCode())
	}
	if policy.DeniedMessage() != defaultDeniedMessage {
		t.Fatalf("DeniedMessage() = %q, want default", policy.DeniedMessage())
	}
}
