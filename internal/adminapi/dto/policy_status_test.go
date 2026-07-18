package dto

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestNewPolicyStatusReturnsReadyForUnappliedPolicy(t *testing.T) {
	const generation int64 = 2
	conditions := []metav1.Condition{
		{
			Type:               string(resource.ConditionAccepted),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: generation,
			Reason:             string(resource.ReasonAccepted),
		},
		{
			Type:               string(resource.ConditionProgrammed),
			Status:             metav1.ConditionFalse,
			ObservedGeneration: generation,
			Reason:             string(resource.ReasonNotApplied),
		},
	}

	got := NewPolicyStatus(generation, 0, conditions)
	want := PolicyStatus{State: ResourceStateReady, Message: messageUnapplied}
	if got != want {
		t.Errorf("NewPolicyStatus(unapplied) = %#v, want %#v", got, want)
	}
}

func TestNewPolicyStatusReturnsReadyWhenOneTargetIsProgrammed(t *testing.T) {
	const generation int64 = 3
	conditions := []metav1.Condition{
		{
			Type:               string(resource.ConditionAccepted),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: generation,
			Reason:             string(resource.ReasonAccepted),
		},
		{
			Type:               string(resource.ConditionResolvedRefs),
			Status:             metav1.ConditionFalse,
			ObservedGeneration: generation,
			Reason:             string(resource.ReasonReferenceNotFound),
		},
		{
			Type:               string(resource.ConditionProgrammed),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: generation,
			Reason:             string(resource.ReasonProgrammed),
		},
	}

	got := NewPolicyStatus(generation, 2, conditions)
	want := PolicyStatus{State: ResourceStateReady, Message: messageReady}
	if got != want {
		t.Errorf("NewPolicyStatus(partially programmed) = %#v, want %#v", got, want)
	}
}

func TestNewPolicyTargetStatusDoesNotRequireAcceptedCondition(t *testing.T) {
	const generation int64 = 3
	conditions := []metav1.Condition{
		{
			Type:               string(resource.ConditionResolvedRefs),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: generation,
			Reason:             string(resource.ReasonResolvedRefs),
		},
		{
			Type:               string(resource.ConditionProgrammed),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: generation,
			Reason:             string(resource.ReasonProgrammed),
		},
	}

	got := NewPolicyTargetStatus(generation, conditions)
	want := ResourceStatus{State: ResourceStateReady, Message: messageReady}
	if got != want {
		t.Errorf("NewPolicyTargetStatus(ready) = %#v, want %#v", got, want)
	}
}

func TestNewPolicyStatusReturnsPendingForUnattachedTargets(t *testing.T) {
	const generation int64 = 4
	conditions := []metav1.Condition{
		{
			Type:               string(resource.ConditionAccepted),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: generation,
			Reason:             string(resource.ReasonAccepted),
		},
		{
			Type:               string(resource.ConditionProgrammed),
			Status:             metav1.ConditionFalse,
			ObservedGeneration: generation,
			Reason:             string(resource.ReasonNotApplied),
			Message:            "Policy is not applied to any target",
		},
	}

	got := NewPolicyStatus(generation, 1, conditions)
	want := PolicyStatus{State: ResourceStatePending, Message: messageTargetNotApplied}
	if got != want {
		t.Errorf("NewPolicyStatus(unattached target) = %#v, want %#v", got, want)
	}

	targetStatus := NewPolicyTargetStatus(generation, conditions[1:])
	if targetStatus.State != ResourceStatePending || targetStatus.Message != messageTargetNotApplied {
		t.Errorf("NewPolicyTargetStatus(unattached target) = %#v, want Pending status", targetStatus)
	}
}

func TestConfigurationAppliedRecognizesActiveAndNotAppliedResults(t *testing.T) {
	const generation int64 = 5
	active := []metav1.Condition{{
		Type:               string(resource.ConditionProgrammed),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: generation,
		Reason:             string(resource.ReasonProgrammed),
	}}
	if !ConfigurationApplied(generation, active) {
		t.Error("ConfigurationApplied(active) = false, want true")
	}

	notApplied := []metav1.Condition{{
		Type:               string(resource.ConditionProgrammed),
		Status:             metav1.ConditionFalse,
		ObservedGeneration: generation,
		Reason:             string(resource.ReasonNotApplied),
	}}
	if !ConfigurationApplied(generation, notApplied) {
		t.Error("ConfigurationApplied(not applied) = false, want true")
	}

	failed := []metav1.Condition{{
		Type:               string(resource.ConditionProgrammed),
		Status:             metav1.ConditionFalse,
		ObservedGeneration: generation,
		Reason:             string(resource.ReasonDeliveryFailed),
	}}
	if ConfigurationApplied(generation, failed) {
		t.Error("ConfigurationApplied(failed) = true, want false")
	}
}
