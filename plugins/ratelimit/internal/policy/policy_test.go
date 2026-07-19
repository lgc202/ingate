package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

func TestBuildChecksBuildsRedisChecksForEveryMatchingPolicy(t *testing.T) {
	route := config.RouteConfig{
		GatewayName: "gw",
		RouteName:   "users",
		Policies: []config.Policy{
			rateLimitPolicy("gateway-limit", "Gateway/gw"),
			rateLimitPolicy("route-limit", "Route/users"),
		},
	}
	req := RequestAttributes{
		GatewayName: "gw",
		RouteName:   "users",
		Headers:     map[string]string{"x-tenant": "acme"},
	}

	checks := BuildChecks(route, req)
	if len(checks) != 2 {
		t.Fatalf("BuildChecks() = %d checks, want 2", len(checks))
	}
	if checks[0].RedisKey == checks[1].RedisKey {
		t.Fatalf("BuildChecks() Redis keys are equal: %q", checks[0].RedisKey)
	}
	for _, segment := range []string{defaultRedisKeyPrefix, "gateway-limit", "Gateway/gw", "tenant"} {
		if !strings.Contains(checks[0].RedisKey, segment) {
			t.Errorf("BuildChecks() gateway Redis key = %q, want readable segment %q", checks[0].RedisKey, segment)
		}
	}
	if !strings.Contains(checks[1].RedisKey, "Route/users") {
		t.Fatalf("BuildChecks() route Redis key = %q, want Route/users scope", checks[1].RedisKey)
	}
	for i, check := range checks {
		if strings.Contains(check.RedisKey, "acme") {
			t.Errorf("BuildChecks()[%d].RedisKey = %q, want sensitive request value hashed", i, check.RedisKey)
		}
	}
	encodedComposite := "6:Header8:x-tenant5:\x01acme"
	digest := sha256.Sum256([]byte(encodedComposite))
	if want := hex.EncodeToString(digest[:]); !strings.HasSuffix(checks[0].RedisKey, want) {
		t.Errorf("BuildChecks()[0].RedisKey = %q, want SHA-256 suffix %q", checks[0].RedisKey, want)
	}
}

func TestBuildChecksUsesStableBucketWhenKeyPartIsMissing(t *testing.T) {
	route := config.RouteConfig{Policies: []config.Policy{rateLimitPolicy("tenant-limit", "Route/users")}}

	first := BuildChecks(route, RequestAttributes{})
	second := BuildChecks(route, RequestAttributes{})
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("BuildChecks(missing dimension) lengths = %d, %d, want 1, 1", len(first), len(second))
	}
	if first[0].RedisKey != second[0].RedisKey {
		t.Fatalf("BuildChecks(missing dimension) keys = %q, %q, want stable bucket", first[0].RedisKey, second[0].RedisKey)
	}
	present := BuildChecks(route, RequestAttributes{Headers: map[string]string{"x-tenant": "acme"}})
	if len(present) != 1 {
		t.Fatalf("BuildChecks(present dimension) = %d checks, want 1", len(present))
	}
	if first[0].RedisKey == present[0].RedisKey {
		t.Fatalf("BuildChecks() missing and present dimensions share Redis key %q", first[0].RedisKey)
	}
}

func TestCompositeKeyHashUsesLengthPrefixedRequestParts(t *testing.T) {
	req := RequestAttributes{
		GatewayName: "gw",
		RouteName:   "users",
		RuleName:    "primary",
		Path:        "/users?token=query-token",
		RemoteAddr:  "10.0.0.1:12345",
		Headers: map[string]string{
			"x-tenant": "acme",
			"cookie":   "session=s1; theme=dark",
		},
	}
	keyHash, ok := compositeKeyHash(req, []config.KeyPart{
		{Type: config.KeyTypeGateway},
		{Type: config.KeyTypeRoute},
		{Type: config.KeyTypeRouteRule},
		{Type: config.KeyTypeHeader, Name: "x-tenant"},
		{Type: config.KeyTypeQuery, Name: "token"},
		{Type: config.KeyTypeCookie, Name: "session"},
		{Type: config.KeyTypeIP},
	})
	if !ok {
		t.Fatal("compositeKeyHash() ok = false, want true")
	}
	encodedComposite := "7:Gateway0:3:\x01gw5:Route0:6:\x01users9:RouteRule0:8:\x01primary6:Header8:x-tenant5:\x01acme5:Query5:token12:\x01query-token6:Cookie7:session3:\x01s12:IP0:9:\x0110.0.0.1"
	digest := sha256.Sum256([]byte(encodedComposite))
	want := hex.EncodeToString(digest[:])
	if keyHash != want {
		t.Fatalf("compositeKeyHash() = %q, want %q", keyHash, want)
	}
}

func TestCompositeKeyHashSeparatesAmbiguousValues(t *testing.T) {
	onePartHash, ok := compositeKeyHash(RequestAttributes{
		Headers: map[string]string{"x-single": "one|Header=two"},
	}, []config.KeyPart{{Type: config.KeyTypeHeader, Name: "x-single"}})
	if !ok {
		t.Fatal("compositeKeyHash(one part) ok = false, want true")
	}
	twoPartHash, ok := compositeKeyHash(RequestAttributes{
		Headers: map[string]string{"x-first": "one", "x-second": "two"},
	}, []config.KeyPart{
		{Type: config.KeyTypeHeader, Name: "x-first"},
		{Type: config.KeyTypeHeader, Name: "x-second"},
	})
	if !ok {
		t.Fatal("compositeKeyHash(two parts) ok = false, want true")
	}
	if onePartHash == twoPartHash {
		t.Fatalf("compositeKeyHash() = %q for distinct type/name/value tuples", onePartHash)
	}
}

func TestKeyValueDoesNotTrustLegacyIdentityDimensions(t *testing.T) {
	req := RequestAttributes{Headers: map[string]string{
		"x-ingate-consumer":      "alice",
		"x-ingate-tenant":        "acme",
		"x-ingate-api-key":       "secret",
		"x-ingate-jwt-claim-sub": "alice",
	}}
	for _, keyType := range []config.KeyType{"Consumer", "Tenant", "APIKey", "JWTClaim"} {
		t.Run(string(keyType), func(t *testing.T) {
			if value, ok := keyValue(req, config.KeyPart{Type: keyType, Name: "sub"}); ok {
				t.Errorf("keyValue(type=%q) = %q, true, want unsupported", keyType, value)
			}
		})
	}
}

func TestDecideRejectsFirstFailClosePolicy(t *testing.T) {
	closePolicy := rateLimitPolicyWithFailurePolicy("close", config.FailurePolicyFailClose)
	closePolicy.Response.StatusCode = 503
	closePolicy.Response.Message = "rate limit unavailable"
	checks := []Check{
		{
			Policy: rateLimitPolicyWithFailurePolicy("open", config.FailurePolicyFailOpen),
			Rule:   config.Rule{Name: "tenant"},
		},
		{
			Policy: closePolicy,
			Rule:   config.Rule{Name: "tenant"},
		},
	}

	decision := Decide(checks, []CheckOutcome{
		{Err: errors.New("redis unavailable")},
		{Err: errors.New("redis unavailable")},
	})
	if decision.Allowed {
		t.Fatal("Decide() allowed = true, want false")
	}
	if decision.StatusCode != closePolicy.Response.StatusCode || decision.Message != closePolicy.Response.Message {
		t.Fatalf("Decide() = %+v, want fail-close response from policy %q", decision, closePolicy.Name)
	}
}

func TestDecideUsesStrictestAllowedQuotaHeaders(t *testing.T) {
	first := rateLimitPolicy("first", "Route/users")
	first.Response.QuotaHeaderEnabled = true
	second := rateLimitPolicy("second", "Route/users")
	second.Response.QuotaHeaderEnabled = true
	decision := Decide(
		[]Check{{Policy: first}, {Policy: second}},
		[]CheckOutcome{
			{Allowed: true, Limit: 10, Remaining: 1, ResetSeconds: 30},
			{Allowed: true, Limit: 10, Remaining: 8, ResetSeconds: 10},
		},
	)
	if !decision.Allowed {
		t.Fatalf("Decide() allowed = false, want true: %+v", decision)
	}
	if got := decision.QuotaHeaders[quotaHeaderRemaining]; got != "1" {
		t.Errorf("Decide() remaining header = %q, want %q", got, "1")
	}
	if got := decision.QuotaHeaders[quotaHeaderReset]; got != "30" {
		t.Errorf("Decide() reset header = %q, want %q", got, "30")
	}
}

func rateLimitPolicy(name, scope string) config.Policy {
	return config.Policy{
		Name:  name,
		Scope: scope,
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

func rateLimitPolicyWithFailurePolicy(name string, failurePolicy config.FailurePolicy) config.Policy {
	policy := rateLimitPolicy(name, "Route/users")
	policy.FailurePolicy = failurePolicy
	return policy
}
