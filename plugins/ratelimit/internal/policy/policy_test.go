package policy

import (
	"errors"
	"testing"
	"time"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

func TestRunnerRejectsAfterQuota(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	runner := NewMemoryRunnerWithClock(func() time.Time { return now })
	route := localRouteConfig()
	req := Request{
		GatewayName: "gw",
		RouteName:   "users",
		Path:        "/users?id=42",
		Headers:     map[string]string{"x-tenant": "acme"},
		RemoteAddr:  "10.0.0.1:12345",
	}

	first := runner.Apply(route, req)
	if !first.Allowed {
		t.Fatalf("first request rejected: %+v", first.Decision)
	}
	if first.QuotaHeaders[quotaHeaderRemaining] != "1" {
		t.Fatalf("first remaining header = %q, want 1", first.QuotaHeaders[quotaHeaderRemaining])
	}
	second := runner.Apply(route, req)
	if !second.Allowed {
		t.Fatalf("second request rejected: %+v", second.Decision)
	}
	third := runner.Apply(route, req)
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

func TestRunnerResetsWindow(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	runner := NewMemoryRunnerWithClock(func() time.Time { return now })
	route := localRouteConfig()
	req := Request{
		GatewayName: "gw",
		RouteName:   "users",
		Path:        "/users?id=42",
		Headers:     map[string]string{"x-tenant": "acme"},
		RemoteAddr:  "10.0.0.1:12345",
	}

	runner.Apply(route, req)
	runner.Apply(route, req)
	now = now.Add(61 * time.Second)
	result := runner.Apply(route, req)
	if !result.Allowed {
		t.Fatalf("request after window reset rejected: %+v", result.Decision)
	}
}

func TestRunnerCollectsGlobalChecks(t *testing.T) {
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

	result := NewMemoryRunner().Apply(route, req)
	if !result.Allowed {
		t.Fatalf("Apply() rejected: %+v", result.Decision)
	}
	if len(result.GlobalChecks) != 2 {
		t.Fatalf("len(GlobalChecks) = %d, want 2", len(result.GlobalChecks))
	}
	if result.GlobalChecks[0].RedisKey == "" {
		t.Fatal("RedisKey is empty")
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

func TestApplyGlobalResultRejectsFirstFailClosePolicy(t *testing.T) {
	checks := []GlobalCheck{
		{
			Policy: globalPolicyWithFailurePolicy("open", config.FailurePolicyFailOpen),
			Rule:   config.Rule{Name: "tenant"},
			Key:    "tenant=acme",
		},
		{
			Policy: globalPolicyWithFailurePolicy("close", config.FailurePolicyFailClose),
			Rule:   config.Rule{Name: "tenant"},
			Key:    "tenant=acme",
		},
	}

	decision, rejected := ApplyGlobalResults(checks, []GlobalOutcome{
		{Err: errors.New("redis unavailable")},
		{Err: errors.New("redis unavailable")},
	})
	if !rejected {
		t.Fatal("rejected = false, want true")
	}
	if decision.Policy.Name != "close" {
		t.Fatalf("rejected policy = %q, want close", decision.Policy.Name)
	}
}

func TestApplyGlobalResultReturnsQuotaHeadersWhenAllowed(t *testing.T) {
	policy := globalPolicy("global")
	policy.Response.QuotaHeaderEnabled = true
	checks := []GlobalCheck{
		{
			Policy: policy,
			Rule:   config.Rule{Name: "tenant"},
			Key:    "tenant=acme",
		},
	}
	decision, rejected := ApplyGlobalResults(checks, []GlobalOutcome{
		{
			Allowed:      true,
			Limit:        10,
			Current:      3,
			ResetSeconds: 30,
		},
	})
	if rejected {
		t.Fatalf("rejected = true: %+v", decision)
	}
	if decision.QuotaHeaders[quotaHeaderRemaining] != "7" {
		t.Fatalf("remaining header = %q, want 7", decision.QuotaHeaders[quotaHeaderRemaining])
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

func globalPolicyWithFailurePolicy(name string, failurePolicy config.FailurePolicy) config.Policy {
	policy := globalPolicy(name)
	policy.FailurePolicy = failurePolicy
	return policy
}
