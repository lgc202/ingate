package ratelimitpolicy

import "testing"

func TestRateLimitPolicyConfigValidateBurst(t *testing.T) {
	tests := []struct {
		name    string
		burst   int
		wantErr bool
	}{
		{name: "default capacity", burst: 0},
		{name: "explicit capacity", burst: 200},
		{name: "negative capacity", burst: -1, wantErr: true},
		{name: "capacity exceeds data plane range", burst: maxPluginInteger + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := RateLimitPolicyConfig{
				Name: "公共限流",
				Rules: []Rule{
					{
						Name:  "default",
						Key:   Key{Parts: []KeyPart{{Type: KeyTypeIP}}},
						Limit: Quota{Requests: 100, WindowSeconds: 60, Burst: tt.burst},
					},
				},
			}

			err := config.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("RateLimitPolicyConfig.Validate(burst=%d) error = %v, want error presence = %t", tt.burst, err, tt.wantErr)
			}
		})
	}
}

func TestKeyPartValidateSupportedTypes(t *testing.T) {
	tests := []struct {
		name    string
		part    KeyPart
		wantErr bool
	}{
		{name: "IP", part: KeyPart{Type: KeyTypeIP}},
		{name: "Header", part: KeyPart{Type: KeyTypeHeader, Name: "x-client"}},
		{name: "invalid Header", part: KeyPart{Type: KeyTypeHeader, Name: "bad header"}, wantErr: true},
		{name: "Query", part: KeyPart{Type: KeyTypeQuery, Name: "client"}},
		{name: "Cookie", part: KeyPart{Type: KeyTypeCookie, Name: "session"}},
		{name: "Route", part: KeyPart{Type: KeyTypeRoute}},
		{name: "Gateway", part: KeyPart{Type: KeyTypeGateway}},
		{name: "RouteRule", part: KeyPart{Type: KeyTypeRouteRule}},
		{name: "Consumer", part: KeyPart{Type: "Consumer"}, wantErr: true},
		{name: "Tenant", part: KeyPart{Type: "Tenant"}, wantErr: true},
		{name: "JWTClaim", part: KeyPart{Type: "JWTClaim", Name: "sub"}, wantErr: true},
		{name: "APIKey", part: KeyPart{Type: "APIKey"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.part.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("KeyPart.Validate(type=%q) error = %v, want error presence = %t", tt.part.Type, err, tt.wantErr)
			}
		})
	}
}

func TestKeyPartValidateClearsUnusedName(t *testing.T) {
	part := KeyPart{Type: KeyTypeRoute, Name: "ignored"}
	if err := part.Validate(); err != nil {
		t.Fatalf("KeyPart.Validate() error = %v", err)
	}
	if part.Name != "" {
		t.Errorf("KeyPart.Validate() name = %q, want empty", part.Name)
	}
}

func TestRateLimitPolicyConfigRejectsNonErrorResponseStatus(t *testing.T) {
	config := RateLimitPolicyConfig{
		Name: "公共限流",
		Rules: []Rule{{
			Name:  "default",
			Key:   Key{Parts: []KeyPart{{Type: KeyTypeIP}}},
			Limit: Quota{Requests: 100, WindowSeconds: 60},
		}},
		Response: Response{StatusCode: 302},
	}
	if err := config.Validate(); err == nil {
		t.Fatal("RateLimitPolicyConfig.Validate(status=302) error = nil, want error")
	}
}
