package accesscontrolpolicy

import (
	"testing"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateAccessControlPolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  *resource.AccessControlPolicy
		wantErr bool
	}{
		{name: "valid without targets", policy: validAccessControlPolicy()},
		{name: "deny all", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.DefaultAction = resource.AccessControlActionDeny
			policy.Spec.Rules = nil
		})},
		{name: "missing display name", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.DisplayName = ""
		}), wantErr: true},
		{name: "duplicate targets", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			ref := resource.PolicyTargetRef{Kind: resource.KindGateway, Name: "gateway-a"}
			policy.Spec.TargetRefs = []resource.PolicyTargetRef{ref, ref}
		}), wantErr: true},
		{name: "invalid default action", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.DefaultAction = "Unknown"
		}), wantErr: true},
		{name: "missing rules", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Rules = nil
		}), wantErr: true},
		{name: "duplicate rule names", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Rules = append(policy.Spec.Rules, policy.Spec.Rules[0])
		}), wantErr: true},
		{name: "invalid rule action", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Rules[0].Action = "Unknown"
		}), wantErr: true},
		{name: "invalid IP condition", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Rules[0].Conditions[0].Value = "not-an-ip"
		}), wantErr: true},
		{name: "IP condition with name", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Rules[0].Conditions[0].Name = "ignored"
		}), wantErr: true},
		{name: "invalid header name", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Rules[0].Conditions[0] = resource.AccessControlCondition{
				Type:  resource.AccessControlConditionTypeHeader,
				Name:  "bad header",
				Value: "value",
			}
		}), wantErr: true},
		{name: "missing condition value", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Rules[0].Conditions[0].Value = ""
		}), wantErr: true},
		{name: "unsupported condition type", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Rules[0].Conditions[0].Type = "Unknown"
		}), wantErr: true},
		{name: "consumer condition is not trusted", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Rules[0].Conditions[0].Type = "Consumer"
		}), wantErr: true},
		{name: "tenant condition is not trusted", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Rules[0].Conditions[0].Type = "Tenant"
		}), wantErr: true},
		{name: "invalid response status", policy: accessControlPolicyWith(func(policy *resource.AccessControlPolicy) {
			policy.Spec.Response.StatusCode = 302
		}), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateAccessControlPolicy(tt.policy)
			if gotErr := len(errs) > 0; gotErr != tt.wantErr {
				t.Errorf("validateAccessControlPolicy(%+v) errors = %v, want error presence = %t", tt.policy.Spec, errs, tt.wantErr)
			}
		})
	}
}

func validAccessControlPolicy() *resource.AccessControlPolicy {
	return &resource.AccessControlPolicy{
		Spec: resource.AccessControlPolicySpec{
			DisplayName:   "access-control",
			DefaultAction: resource.AccessControlActionAllow,
			Rules: []resource.AccessControlRule{
				{
					Name:   "office",
					Action: resource.AccessControlActionAllow,
					Conditions: []resource.AccessControlCondition{
						{Type: resource.AccessControlConditionTypeIP, Value: "10.0.0.0/8"},
					},
				},
			},
		},
	}
}

func accessControlPolicyWith(change func(*resource.AccessControlPolicy)) *resource.AccessControlPolicy {
	policy := validAccessControlPolicy()
	change(policy)
	return policy
}
