package ratelimit

import "testing"

func TestParseRouteConfigDefaultsSchemaVersion(t *testing.T) {
	cfg, err := ParseRouteConfig([]byte(`{"gatewayName":"gw","routeName":"route","bindings":[]}`))
	if err != nil {
		t.Fatalf("ParseRouteConfig() error = %v", err)
	}
	if cfg.SchemaVersion != schemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", cfg.SchemaVersion, schemaVersion)
	}
}

func TestParseRouteConfigAcceptsTypedStructEnvelope(t *testing.T) {
	cfg, err := ParseRouteConfig([]byte(`{
		"typeUrl":"type.googleapis.com/ingate.extensions.filters.http.ratelimit.v1.RouteConfig",
		"value":{"schemaVersion":"v1","gatewayName":"gw","routeName":"route","bindings":[]}
	}`))
	if err != nil {
		t.Fatalf("ParseRouteConfig() error = %v", err)
	}
	if cfg.GatewayName != "gw" || cfg.RouteName != "route" {
		t.Fatalf("route identity = %s/%s, want gw/route", cfg.GatewayName, cfg.RouteName)
	}
}

func TestParsePluginConfigRejectsUnknownSchemaVersion(t *testing.T) {
	_, err := ParsePluginConfig([]byte(`{"schemaVersion":"v2"}`))
	if err == nil {
		t.Fatalf("ParsePluginConfig() error = nil, want unsupported version")
	}
}

func TestPolicyResponseDefaults(t *testing.T) {
	var policy Policy
	if policy.RejectedStatusCode() != defaultRejectedStatusCode {
		t.Fatalf("RejectedStatusCode() = %d, want default", policy.RejectedStatusCode())
	}
	if policy.RejectedMessage() != defaultRejectedMessage {
		t.Fatalf("RejectedMessage() = %q, want default", policy.RejectedMessage())
	}
	if !policy.FailOpen() {
		t.Fatalf("FailOpen() = false, want true for empty failure policy")
	}
}
