package acl

import "testing"

func TestParsePluginConfigDefaultsSchemaVersion(t *testing.T) {
	cfg, err := ParsePluginConfig([]byte(`{"routes":[]}`))
	if err != nil {
		t.Fatalf("ParsePluginConfig() error = %v", err)
	}
	if cfg.SchemaVersion != schemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", cfg.SchemaVersion, schemaVersion)
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
