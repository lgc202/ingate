package ratelimit

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
		`{"routes":[],"unknown":"value"}`,
		`{"routes":[{"gatewayName":"gw","routeName":"route","bindings":[{"name":"binding","target":{"kind":"Route","name":"route"},"policies":[{"name":"policy","mode":"Global","rules":[],"unknown":"value"}]}]}]}`,
		`{"routes":[]} {}`,
	} {
		if _, err := ParsePluginConfig([]byte(data)); err == nil {
			t.Errorf("ParsePluginConfig(%s) error = nil, want rejection", data)
		}
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
