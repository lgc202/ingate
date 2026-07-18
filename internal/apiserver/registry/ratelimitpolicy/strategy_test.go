package ratelimitpolicy

import (
	"testing"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateRateLimitPolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  *resource.RateLimitPolicy
		wantErr bool
	}{
		{name: "valid without targets", policy: validRateLimitPolicy()},
		{name: "missing display name", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.DisplayName = ""
		}), wantErr: true},
		{name: "duplicate targets", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			ref := resource.PolicyTargetRef{Kind: resource.KindRoute, Name: "route-a"}
			policy.Spec.TargetRefs = []resource.PolicyTargetRef{ref, ref}
		}), wantErr: true},
		{name: "missing rules", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules = nil
		}), wantErr: true},
		{name: "duplicate rule names", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules = append(policy.Spec.Rules, policy.Spec.Rules[0])
		}), wantErr: true},
		{name: "missing key parts", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Key.Parts = nil
		}), wantErr: true},
		{name: "invalid header key name", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Key.Parts = []resource.RateLimitKeyPart{{Type: resource.RateLimitKeyTypeHeader, Name: "bad header"}}
		}), wantErr: true},
		{name: "IP key with name", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Key.Parts[0].Name = "ignored"
		}), wantErr: true},
		{name: "unsupported key type", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Key.Parts = []resource.RateLimitKeyPart{{Type: "Unknown"}}
		}), wantErr: true},
		{name: "consumer key type is not trusted", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Key.Parts = []resource.RateLimitKeyPart{{Type: "Consumer"}}
		}), wantErr: true},
		{name: "tenant key type is not trusted", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Key.Parts = []resource.RateLimitKeyPart{{Type: "Tenant"}}
		}), wantErr: true},
		{name: "JWT claim key type is not trusted", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Key.Parts = []resource.RateLimitKeyPart{{Type: "JWTClaim", Name: "sub"}}
		}), wantErr: true},
		{name: "API key type is not trusted", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Key.Parts = []resource.RateLimitKeyPart{{Type: "APIKey"}}
		}), wantErr: true},
		{name: "invalid requests", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Limit.Requests = 0
		}), wantErr: true},
		{name: "invalid window", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Limit.WindowSeconds = 0
		}), wantErr: true},
		{name: "invalid burst", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Limit.Burst = -1
		}), wantErr: true},
		{name: "integer exceeds data plane range", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Rules[0].Limit.Requests = maxPluginInteger + 1
		}), wantErr: true},
		{name: "invalid response status", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.Response.StatusCode = 302
		}), wantErr: true},
		{name: "invalid failure policy", policy: rateLimitPolicyWith(func(policy *resource.RateLimitPolicy) {
			policy.Spec.FailurePolicy = "Unknown"
		}), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateRateLimitPolicy(tt.policy)
			if gotErr := len(errs) > 0; gotErr != tt.wantErr {
				t.Errorf("validateRateLimitPolicy(%+v) errors = %v, want error presence = %t", tt.policy.Spec, errs, tt.wantErr)
			}
		})
	}
}

func validRateLimitPolicy() *resource.RateLimitPolicy {
	return &resource.RateLimitPolicy{
		Spec: resource.RateLimitPolicySpec{
			DisplayName: "rate-limit",
			Rules: []resource.RateLimitRule{
				{
					Name:  "per-ip",
					Key:   resource.RateLimitKey{Parts: []resource.RateLimitKeyPart{{Type: resource.RateLimitKeyTypeIP}}},
					Limit: resource.RateLimitQuota{Requests: 100, WindowSeconds: 60},
				},
			},
		},
	}
}

func rateLimitPolicyWith(change func(*resource.RateLimitPolicy)) *resource.RateLimitPolicy {
	policy := validRateLimitPolicy()
	change(policy)
	return policy
}
