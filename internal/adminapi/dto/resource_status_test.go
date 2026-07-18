package dto

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestNewResourceStatus(t *testing.T) {
	const generation int64 = 3

	tests := []struct {
		name       string
		conditions []metav1.Condition
		want       ResourceStatus
	}{
		{
			name: "missing conditions",
			want: ResourceStatus{State: ResourceStatePending, Message: messagePending},
		},
		{
			name: "stale accepted error",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionFalse, resource.ReasonInvalidSpec, generation-1),
			},
			want: ResourceStatus{State: ResourceStatePending, Message: messagePending},
		},
		{
			name: "accepted error",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionFalse, resource.ReasonInvalidSpec, generation),
			},
			want: ResourceStatus{State: ResourceStateError, Message: messageInvalidSpec},
		},
		{
			name: "accepted unknown",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionUnknown, resource.ReasonPending, generation),
			},
			want: ResourceStatus{State: ResourceStatePending, Message: messagePending},
		},
		{
			name: "reference error",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionResolvedRefs, metav1.ConditionFalse, resource.ReasonReferenceNotFound, generation),
			},
			want: ResourceStatus{State: ResourceStateError, Message: messageReferenceNotFound},
		},
		{
			name: "invalid reference",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionResolvedRefs, metav1.ConditionFalse, resource.ReasonInvalidReference, generation),
			},
			want: ResourceStatus{State: ResourceStateError, Message: messageInvalidReference},
		},
		{
			name: "references pending",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionResolvedRefs, metav1.ConditionUnknown, resource.ReasonPending, generation),
			},
			want: ResourceStatus{State: ResourceStatePending, Message: messageCheckingReferences},
		},
		{
			name: "programming pending",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionProgrammed, metav1.ConditionFalse, resource.ReasonPending, generation),
			},
			want: ResourceStatus{State: ResourceStatePending, Message: messageProgramming},
		},
		{
			name: "programming rejected",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionProgrammed, metav1.ConditionFalse, resource.ReasonRejected, generation),
			},
			want: ResourceStatus{State: ResourceStateError, Message: messageRejected},
		},
		{
			name: "delivery failed",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionProgrammed, metav1.ConditionFalse, resource.ReasonDeliveryFailed, generation),
			},
			want: ResourceStatus{State: ResourceStateError, Message: messageDeliveryFailed},
		},
		{
			name: "programming compile failed",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionProgrammed, metav1.ConditionFalse, resource.ReasonCompileFailed, generation),
			},
			want: ResourceStatus{State: ResourceStateError, Message: messageCompileFailed},
		},
		{
			name: "programming failed with unknown reason",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionProgrammed, metav1.ConditionFalse, resource.ConditionReason("UnknownFailure"), generation),
			},
			want: ResourceStatus{State: ResourceStateError, Message: messageCompileFailed},
		},
		{
			name: "programming unknown",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionProgrammed, metav1.ConditionUnknown, resource.ReasonPending, generation),
			},
			want: ResourceStatus{State: ResourceStatePending, Message: messageProgramming},
		},
		{
			name: "stale programming rejection",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionProgrammed, metav1.ConditionFalse, resource.ReasonRejected, generation-1),
			},
			want: ResourceStatus{State: ResourceStatePending, Message: messageProgramming},
		},
		{
			name: "ready without resolved refs condition",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionProgrammed, metav1.ConditionTrue, resource.ReasonProgrammed, generation),
			},
			want: ResourceStatus{State: ResourceStateReady, Message: messageReady},
		},
		{
			name: "ready",
			conditions: []metav1.Condition{
				newCondition(resource.ConditionAccepted, metav1.ConditionTrue, resource.ReasonAccepted, generation),
				newCondition(resource.ConditionResolvedRefs, metav1.ConditionTrue, resource.ReasonResolvedRefs, generation),
				newCondition(resource.ConditionProgrammed, metav1.ConditionTrue, resource.ReasonProgrammed, generation),
			},
			want: ResourceStatus{State: ResourceStateReady, Message: messageReady},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewResourceStatus(generation, tt.conditions)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("NewResourceStatus(%d, %v) mismatch (-want +got):\n%s", generation, tt.conditions, diff)
			}
		})
	}
}

func TestNewDisabledResourceStatus(t *testing.T) {
	want := ResourceStatus{State: ResourceStateDisabled, Message: messageDisabled}
	got := NewDisabledResourceStatus()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("NewDisabledResourceStatus() mismatch (-want +got):\n%s", diff)
	}
}

func newCondition(conditionType resource.ConditionType, status metav1.ConditionStatus, reason resource.ConditionReason, generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		ObservedGeneration: generation,
		Reason:             string(reason),
	}
}
