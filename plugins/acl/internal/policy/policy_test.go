package policy

import (
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/acl"
)

func TestEvaluateDeniesMatchingRule(t *testing.T) {
	route := routeConfig(config.Policy{Rules: riskRules()})
	req := RequestAttributes{Headers: map[string]string{"x-risk-level": "high"}}

	decision := route.Evaluate(req)
	if decision.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if decision.StatusCode != 403 {
		t.Fatalf("StatusCode = %d, want 403", decision.StatusCode)
	}
}

func TestEvaluateAllowsUnmatchedRequestByDefault(t *testing.T) {
	route := routeConfig(config.Policy{Rules: riskRules()})

	decision := route.Evaluate(RequestAttributes{Headers: map[string]string{"x-risk-level": "low"}})
	if !decision.Allowed {
		t.Fatalf("Allowed = false: %+v", decision)
	}
}

func TestEvaluateSupportsIPCidrCondition(t *testing.T) {
	route := routeConfig(config.Policy{
		Rules: []config.Rule{
			{
				Name:   "allow-office",
				Action: config.ActionAllow,
				Conditions: []config.Condition{
					{Type: config.ConditionTypeIP, Value: "10.0.0.0/8"},
				},
			},
		},
		DefaultAction: config.ActionDeny,
	})

	decision := route.Evaluate(RequestAttributes{RemoteAddr: "10.1.2.3:12345"})
	if !decision.Allowed {
		t.Fatalf("Allowed = false: %+v", decision)
	}
}

func TestEvaluateDoesNotTrustLegacyIdentityConditions(t *testing.T) {
	req := RequestAttributes{Headers: map[string]string{
		"x-ingate-consumer": "alice",
		"x-ingate-tenant":   "acme",
	}}
	for _, tt := range []struct {
		conditionType config.ConditionType
		value         string
	}{
		{conditionType: "Consumer", value: "alice"},
		{conditionType: "Tenant", value: "acme"},
	} {
		t.Run(string(tt.conditionType), func(t *testing.T) {
			condition := config.Condition{Type: tt.conditionType, Value: tt.value}
			if conditionMatches(condition, req) {
				t.Errorf("conditionMatches(type=%q) = true, want false", tt.conditionType)
			}
		})
	}
}

func routeConfig(policy config.Policy) Route {
	return Route{policies: []config.Policy{policy}}
}

func riskRules() []config.Rule {
	return []config.Rule{
		{
			Name:   "block-risk",
			Action: config.ActionDeny,
			Conditions: []config.Condition{
				{Type: config.ConditionTypeHeader, Name: "x-risk-level", Value: "high"},
			},
		},
	}
}
