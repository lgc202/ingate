package ratelimitpolicy

import (
	"encoding/json"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestResponseJSONPreservesDisabledQuotaHeaders(t *testing.T) {
	data, err := json.Marshal(Response{QuotaHeaderEnabled: false})
	if err != nil {
		t.Fatalf("json.Marshal(Response) error = %v", err)
	}
	if !strings.Contains(string(data), `"quotaHeaderEnabled":false`) {
		t.Errorf("json.Marshal(Response{QuotaHeaderEnabled:false}) = %s, want explicit false field", data)
	}
}

func TestPolicyFromResourceDoesNotHideFailedDisable(t *testing.T) {
	policy := &resource.RateLimitPolicy{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Status: resource.PolicyStatus{Conditions: []metav1.Condition{
			{
				Type:               string(resource.ConditionAccepted),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 2,
				Reason:             string(resource.ReasonAccepted),
			},
			{
				Type:               string(resource.ConditionResolvedRefs),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 2,
				Reason:             string(resource.ReasonResolvedRefs),
			},
			{
				Type:               string(resource.ConditionProgrammed),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: 2,
				Reason:             string(resource.ReasonDeliveryFailed),
			},
		}},
	}

	response := policyFromResource(policy, nil)
	if response.Status.State != admindto.ResourceStateError {
		t.Errorf("policyFromResource(failed disable).Status.State = %q, want %q", response.Status.State, admindto.ResourceStateError)
	}
}

func TestPolicyFromResourceMaterializesExecutionDefaults(t *testing.T) {
	response := policyFromResource(&resource.RateLimitPolicy{}, nil)

	if response.Response.StatusCode != defaultRejectedStatusCode {
		t.Errorf("policyFromResource().Response.StatusCode = %d, want %d", response.Response.StatusCode, defaultRejectedStatusCode)
	}
	if response.Response.Message != defaultRejectedMessage {
		t.Errorf("policyFromResource().Response.Message = %q, want %q", response.Response.Message, defaultRejectedMessage)
	}
	if response.FailurePolicy != defaultFailurePolicy {
		t.Errorf("policyFromResource().FailurePolicy = %q, want %q", response.FailurePolicy, defaultFailurePolicy)
	}
}
