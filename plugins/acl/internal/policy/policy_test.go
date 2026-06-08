package policy

import (
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/acl"
)

func TestRunnerDeniesMatchingRule(t *testing.T) {
	route := config.RouteConfig{
		Rules: []config.Rule{
			{
				Name:   "block-risk",
				Action: config.ActionDeny,
				Conditions: []config.Condition{
					{Type: config.ConditionTypeHeader, Name: "x-risk-level", Value: "high"},
				},
			},
		},
	}
	req := Request{Headers: map[string]string{"x-risk-level": "high"}}

	decision := NewRunner().Apply(route, req)
	if decision.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if decision.StatusCode != 403 {
		t.Fatalf("StatusCode = %d, want 403", decision.StatusCode)
	}
}

func TestRunnerAllowsUnmatchedRequestByDefault(t *testing.T) {
	route := config.RouteConfig{
		Rules: []config.Rule{
			{
				Name:   "block-risk",
				Action: config.ActionDeny,
				Conditions: []config.Condition{
					{Type: config.ConditionTypeHeader, Name: "x-risk-level", Value: "high"},
				},
			},
		},
	}

	decision := NewRunner().Apply(route, Request{Headers: map[string]string{"x-risk-level": "low"}})
	if !decision.Allowed {
		t.Fatalf("Allowed = false: %+v", decision)
	}
}

func TestRunnerSupportsIPCidrCondition(t *testing.T) {
	route := config.RouteConfig{
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
	}

	decision := NewRunner().Apply(route, Request{RemoteAddr: "10.1.2.3:12345"})
	if !decision.Allowed {
		t.Fatalf("Allowed = false: %+v", decision)
	}
}
