package accesscontrolpolicy

import (
	"testing"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestPolicyFromResourceMaterializesExecutionDefaults(t *testing.T) {
	response := policyFromResource(&resource.AccessControlPolicy{}, nil)

	if response.DefaultAction != ActionAllow {
		t.Errorf("policyFromResource().DefaultAction = %q, want %q", response.DefaultAction, ActionAllow)
	}
	if response.Response.StatusCode != defaultDeniedStatusCode {
		t.Errorf("policyFromResource().Response.StatusCode = %d, want %d", response.Response.StatusCode, defaultDeniedStatusCode)
	}
	if response.Response.Message != defaultDeniedResponseText {
		t.Errorf("policyFromResource().Response.Message = %q, want %q", response.Response.Message, defaultDeniedResponseText)
	}
}
