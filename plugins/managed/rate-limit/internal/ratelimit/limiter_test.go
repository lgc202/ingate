package ratelimit

import (
	"testing"
	"time"

	"github.com/lgc202/ingate/plugins/managed/rate-limit/internal/config"
)

func TestLocalLimiterRejectsAfterQuota(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	limiter := NewLocalLimiterWithClock(func() time.Time { return now })
	route := localRouteConfig()
	req := Request{
		GatewayName: "gw",
		RouteName:   "users",
		Path:        "/users?id=42",
		Headers:     map[string]string{"x-tenant": "acme"},
		RemoteAddr:  "10.0.0.1:12345",
	}

	first := limiter.Evaluate(route, req)
	if !first.Allowed {
		t.Fatalf("first request rejected: %+v", first.Decision)
	}
	if first.QuotaHeaders[quotaHeaderRemaining] != "1" {
		t.Fatalf("first remaining header = %q, want 1", first.QuotaHeaders[quotaHeaderRemaining])
	}
	second := limiter.Evaluate(route, req)
	if !second.Allowed {
		t.Fatalf("second request rejected: %+v", second.Decision)
	}
	third := limiter.Evaluate(route, req)
	if third.Allowed {
		t.Fatalf("third request allowed, want rejected")
	}
	if third.Decision.StatusCode != 429 {
		t.Fatalf("StatusCode = %d, want 429", third.Decision.StatusCode)
	}
	if third.Decision.QuotaHeaders[quotaHeaderRemaining] != "0" {
		t.Fatalf("remaining header = %q, want 0", third.Decision.QuotaHeaders[quotaHeaderRemaining])
	}
}

func TestLocalLimiterResetsWindow(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	limiter := NewLocalLimiterWithClock(func() time.Time { return now })
	route := localRouteConfig()
	req := Request{
		GatewayName: "gw",
		RouteName:   "users",
		Path:        "/users?id=42",
		Headers:     map[string]string{"x-tenant": "acme"},
		RemoteAddr:  "10.0.0.1:12345",
	}

	limiter.Evaluate(route, req)
	limiter.Evaluate(route, req)
	now = now.Add(61 * time.Second)
	result := limiter.Evaluate(route, req)
	if !result.Allowed {
		t.Fatalf("request after window reset rejected: %+v", result.Decision)
	}
}

func TestLocalLimiterCollectsGlobalChecks(t *testing.T) {
	route := config.RouteConfig{
		GatewayName: "gw",
		RouteName:   "users",
		Bindings: []config.Binding{
			{
				Name: "binding",
				Policies: []config.Policy{
					globalPolicy("policy-a"),
					globalPolicy("policy-b"),
				},
			},
		},
	}
	req := Request{
		GatewayName: "gw",
		RouteName:   "users",
		Headers:     map[string]string{"x-tenant": "acme"},
	}

	result := NewLocalLimiter().Evaluate(route, req)
	if !result.Allowed {
		t.Fatalf("Evaluate() rejected: %+v", result.Decision)
	}
	if len(result.RedisChecks) != 2 {
		t.Fatalf("len(RedisChecks) = %d, want 2", len(result.RedisChecks))
	}
	if result.RedisChecks[0].RedisStore != "redis-main" {
		t.Fatalf("RedisStore = %q, want redis-main", result.RedisChecks[0].RedisStore)
	}
}

func TestCompositeKeyUsesRequestParts(t *testing.T) {
	req := Request{
		GatewayName: "gw",
		RouteName:   "users",
		RuleName:    "primary",
		Path:        "/users?token=query-token",
		RemoteAddr:  "10.0.0.1:12345",
		Headers: map[string]string{
			"x-tenant":          "acme",
			"x-ingate-consumer": "alice",
			"cookie":            "session=s1; theme=dark",
		},
	}
	key, ok := compositeKey(req, []config.KeyPart{
		{Type: config.KeyTypeGateway},
		{Type: config.KeyTypeRoute},
		{Type: config.KeyTypeHeader, Name: "x-tenant"},
		{Type: config.KeyTypeQuery, Name: "token"},
		{Type: config.KeyTypeCookie, Name: "session"},
		{Type: config.KeyTypeConsumer},
		{Type: config.KeyTypeIP},
	})
	if !ok {
		t.Fatalf("compositeKey() ok = false, want true")
	}
	want := "Gateway=gw|Route=users/primary|Header=acme|Query=query-token|Cookie=s1|Consumer=alice|IP=10.0.0.1"
	if key != want {
		t.Fatalf("key = %q, want %q", key, want)
	}
}

func localRouteConfig() config.RouteConfig {
	return config.RouteConfig{
		GatewayName: "gw",
		RouteName:   "users",
		Bindings: []config.Binding{
			{
				Name: "binding",
				Policies: []config.Policy{
					{
						Name: "local",
						Mode: config.ModeLocal,
						Rules: []config.Rule{
							{
								Name: "tenant",
								Key: []config.KeyPart{
									{Type: config.KeyTypeHeader, Name: "x-tenant"},
								},
								Limit: config.Quota{Requests: 2, WindowSeconds: 60},
							},
						},
						Response: config.Response{QuotaHeaderEnabled: true},
					},
				},
			},
		},
	}
}

func globalPolicy(name string) config.Policy {
	return config.Policy{
		Name: name,
		Mode: config.ModeGlobal,
		Global: &config.Global{
			RedisRef: "redis-main",
			Prefix:   "ingate-test",
		},
		Rules: []config.Rule{
			{
				Name: "tenant",
				Key: []config.KeyPart{
					{Type: config.KeyTypeHeader, Name: "x-tenant"},
				},
				Limit: config.Quota{Requests: 2, WindowSeconds: 60},
			},
		},
	}
}
