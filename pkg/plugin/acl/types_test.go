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
	var route RouteConfig
	if route.DeniedStatusCode() != defaultDeniedStatusCode {
		t.Fatalf("DeniedStatusCode() = %d, want default", route.DeniedStatusCode())
	}
	if route.DeniedMessage() != defaultDeniedMessage {
		t.Fatalf("DeniedMessage() = %q, want default", route.DeniedMessage())
	}
}
