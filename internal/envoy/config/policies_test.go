package config

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestPolicyConfigDeduplicatesPolicyAndUsesMostSpecificScope(t *testing.T) {
	listener := listenerKey{address: "0.0.0.0", port: 8080, protocol: gatewayv1.ProtocolHTTP}
	policy := testRateLimitPolicy("shared", []gatewayv1.PolicyTargetRef{
		{Kind: gatewayv1.KindGateway, Name: "gateway"},
		{Kind: gatewayv1.KindRoute, Name: "route"},
	})
	context := newPolicyTestContext(listener, policy)

	configs := context.buildPolicyConfigs()
	routes := configs[listener].rateLimit.Routes
	if len(routes) != 1 {
		t.Fatalf("len(Routes) = %d, want 1", len(routes))
	}
	if len(routes[0].Policies) != 1 {
		t.Fatalf("len(Policies) = %d, want 1", len(routes[0].Policies))
	}
	if scope := routes[0].Policies[0].Scope; scope != "Route/route" {
		t.Fatalf("Scope = %q, want %q", scope, "Route/route")
	}
	programmedTargets := context.programmedPolicyTargets()
	wantTargets := []ProgrammedPolicyTarget{
		{
			Policy: ResourceGeneration{Kind: gatewayv1.KindRateLimitPolicy, Name: policy.Name},
			Target: ResourceGeneration{Kind: gatewayv1.KindGateway, Name: "gateway"},
		},
		{
			Policy: ResourceGeneration{Kind: gatewayv1.KindRateLimitPolicy, Name: policy.Name},
			Target: ResourceGeneration{Kind: gatewayv1.KindRoute, Name: "route"},
		},
	}
	if !slices.Equal(programmedTargets, wantTargets) {
		t.Fatalf("compileContext.programmedPolicyTargets() = %v, want %v", programmedTargets, wantTargets)
	}
}

func TestPolicyConfigKeepsValidTargetsWhenAnotherTargetIsMissing(t *testing.T) {
	listener := listenerKey{address: "0.0.0.0", port: 8080, protocol: gatewayv1.ProtocolHTTP}
	policy := testRateLimitPolicy("partial", []gatewayv1.PolicyTargetRef{
		{Kind: gatewayv1.KindGateway, Name: "missing"},
		{Kind: gatewayv1.KindRoute, Name: "route"},
	})
	context := newPolicyTestContext(listener, policy)

	configs := context.buildPolicyConfigs()
	if len(configs[listener].rateLimit.Routes) != 1 {
		t.Fatal("valid Route target was not compiled")
	}
	if len(context.diagnostics) != 1 {
		t.Fatalf("len(Diagnostics) = %d, want 1", len(context.diagnostics))
	}
	diagnostic := context.diagnostics[0]
	if diagnostic.Severity != SeverityWarning || diagnostic.Reason != ReasonReferenceNotFound {
		t.Fatalf("Diagnostic = %+v, want reference warning", diagnostic)
	}
}

func TestAccessControlPolicyRejectsUntrustedIdentityConditions(t *testing.T) {
	for _, conditionType := range []gatewayv1.AccessControlConditionType{"Consumer", "Tenant"} {
		t.Run(string(conditionType), func(t *testing.T) {
			context := &compileContext{diagnosticSet: make(map[string]bool)}
			policy := &gatewayv1.AccessControlPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "access-control"},
				Spec: gatewayv1.AccessControlPolicySpec{
					DefaultAction: gatewayv1.AccessControlActionAllow,
					Rules: []gatewayv1.AccessControlRule{{
						Name:   "trusted",
						Action: gatewayv1.AccessControlActionAllow,
						Conditions: []gatewayv1.AccessControlCondition{{
							Type:  conditionType,
							Value: "value",
						}},
					}},
				},
			}

			if _, valid := context.accessControlPolicy(policy); valid {
				t.Errorf("compileContext.accessControlPolicy(type=%q) valid = true, want false", conditionType)
			}
		})
	}
}

func TestRateLimitPolicyRejectsUntrustedIdentityKeyTypes(t *testing.T) {
	for _, keyType := range []gatewayv1.RateLimitKeyType{"Consumer", "Tenant", "JWTClaim", "APIKey"} {
		t.Run(string(keyType), func(t *testing.T) {
			context := &compileContext{diagnosticSet: make(map[string]bool)}
			policy := testRateLimitPolicy("rate-limit", nil)
			policy.Spec.Rules[0].Key.Parts[0] = gatewayv1.RateLimitKeyPart{Type: keyType, Name: "value"}

			if _, valid := context.rateLimitPolicy(policy); valid {
				t.Errorf("compileContext.rateLimitPolicy(type=%q) valid = true, want false", keyType)
			}
		})
	}
}

func TestPolicyConfigRejectsNamesOnUnnamedDimensions(t *testing.T) {
	context := &compileContext{diagnosticSet: make(map[string]bool)}
	accessPolicy := &gatewayv1.AccessControlPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "access-control"},
		Spec: gatewayv1.AccessControlPolicySpec{
			DefaultAction: gatewayv1.AccessControlActionAllow,
			Rules: []gatewayv1.AccessControlRule{{
				Name:   "office",
				Action: gatewayv1.AccessControlActionAllow,
				Conditions: []gatewayv1.AccessControlCondition{{
					Type:  gatewayv1.AccessControlConditionTypeIP,
					Name:  "ignored",
					Value: "10.0.0.1",
				}},
			}},
		},
	}
	if _, valid := context.accessControlPolicy(accessPolicy); valid {
		t.Error("compileContext.accessControlPolicy(IP name) valid = true, want false")
	}

	context = &compileContext{diagnosticSet: make(map[string]bool)}
	ratePolicy := testRateLimitPolicy("rate-limit", nil)
	ratePolicy.Spec.Rules[0].Key.Parts[0].Name = "ignored"
	if _, valid := context.rateLimitPolicy(ratePolicy); valid {
		t.Error("compileContext.rateLimitPolicy(IP name) valid = true, want false")
	}
}

func TestPolicyConfigRejectsNonErrorResponseStatus(t *testing.T) {
	context := &compileContext{diagnosticSet: make(map[string]bool)}
	accessPolicy := &gatewayv1.AccessControlPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "access-control"},
		Spec: gatewayv1.AccessControlPolicySpec{
			DefaultAction: gatewayv1.AccessControlActionDeny,
			Response:      gatewayv1.AccessControlDenyResponse{StatusCode: 302},
		},
	}
	if _, valid := context.accessControlPolicy(accessPolicy); valid {
		t.Error("compileContext.accessControlPolicy(status=302) valid = true, want false")
	}

	context = &compileContext{diagnosticSet: make(map[string]bool)}
	ratePolicy := testRateLimitPolicy("rate-limit", nil)
	ratePolicy.Spec.Response.StatusCode = 302
	if _, valid := context.rateLimitPolicy(ratePolicy); valid {
		t.Error("compileContext.rateLimitPolicy(status=302) valid = true, want false")
	}
}

func newPolicyTestContext(listener listenerKey, policy *gatewayv1.RateLimitPolicy) *compileContext {
	return &compileContext{
		gateways: map[string]*gatewayv1.Gateway{
			"gateway": {ObjectMeta: metav1.ObjectMeta{Name: "gateway"}},
		},
		routes: map[string]*gatewayv1.Route{
			"route": {ObjectMeta: metav1.ObjectMeta{Name: "route"}},
		},
		rateLimitPolicies: map[string]*gatewayv1.RateLimitPolicy{
			policy.Name: policy,
		},
		accessControlPolicies: make(map[string]*gatewayv1.AccessControlPolicy),
		routeAttachments: []routeAttachment{
			{listenerKey: listener, gatewayID: "gateway", routeID: "route", ruleName: "rule"},
		},
		diagnosticSet: make(map[string]bool),
		policyTargets: make(map[ProgrammedPolicyTarget]bool),
	}
}

func testRateLimitPolicy(name string, targets []gatewayv1.PolicyTargetRef) *gatewayv1.RateLimitPolicy {
	return &gatewayv1.RateLimitPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1.RateLimitPolicySpec{
			Enabled:    true,
			TargetRefs: targets,
			Rules: []gatewayv1.RateLimitRule{
				{
					Name: "client",
					Key: gatewayv1.RateLimitKey{
						Parts: []gatewayv1.RateLimitKeyPart{{Type: gatewayv1.RateLimitKeyTypeIP}},
					},
					Limit: gatewayv1.RateLimitQuota{Requests: 100, WindowSeconds: 60},
				},
			},
		},
	}
}
