package accesscontrolpolicy

import "testing"

func TestConditionValidateSupportedTypes(t *testing.T) {
	tests := []struct {
		name      string
		condition Condition
		wantErr   bool
	}{
		{name: "IP", condition: Condition{Type: ConditionTypeIP, Value: "10.0.0.0/8"}},
		{name: "invalid IP", condition: Condition{Type: ConditionTypeIP, Value: "office"}, wantErr: true},
		{name: "Header", condition: Condition{Type: ConditionTypeHeader, Name: "x-client", Value: "trusted"}},
		{name: "invalid Header", condition: Condition{Type: ConditionTypeHeader, Name: "bad header", Value: "trusted"}, wantErr: true},
		{name: "Consumer", condition: Condition{Type: "Consumer", Value: "alice"}, wantErr: true},
		{name: "Tenant", condition: Condition{Type: "Tenant", Value: "acme"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.condition.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("Condition.Validate(type=%q) error = %v, want error presence = %t", tt.condition.Type, err, tt.wantErr)
			}
		})
	}
}

func TestConditionValidateClearsUnusedIPName(t *testing.T) {
	condition := Condition{Type: ConditionTypeIP, Name: "ignored", Value: "10.0.0.1"}
	if err := condition.Validate(); err != nil {
		t.Fatalf("Condition.Validate() error = %v", err)
	}
	if condition.Name != "" {
		t.Errorf("Condition.Validate() name = %q, want empty", condition.Name)
	}
}

func TestAccessControlPolicyConfigRejectsNonErrorResponseStatus(t *testing.T) {
	config := AccessControlPolicyConfig{
		Name:          "访问控制",
		DefaultAction: ActionDeny,
		Response:      DenyResponse{StatusCode: 302},
	}
	if err := config.Validate(); err == nil {
		t.Fatal("AccessControlPolicyConfig.Validate(status=302) error = nil, want error")
	}
}
