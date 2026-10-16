package deletion

import (
	"errors"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

func TestConditionsAreStableAndRecover(t *testing.T) {
	namespace := &corev1.Namespace{
		Status: corev1.NamespaceStatus{
			Conditions: []corev1.NamespaceCondition{{
				Type:   "Custom",
				Status: corev1.ConditionTrue,
				Reason: "Keep",
			}},
		},
	}
	var updater namespaceConditionUpdater
	updater.ProcessDeleteContentErr(errors.New("content deletion unavailable"))
	if !updater.Update(namespace) {
		t.Fatal("Update(first failure) = false, want conditions installed")
	}

	before := namespace.DeepCopy()
	if updater.Update(namespace) {
		t.Error("Update(same failure) = true, want unchanged conditions")
	}

	condition := getCondition(namespace.Status.Conditions, corev1.NamespaceDeletionContentFailure)
	previous := getCondition(before.Status.Conditions, corev1.NamespaceDeletionContentFailure)
	if condition == nil || previous == nil {
		t.Fatalf("Update(failure) missing content failure condition: current %v, previous %v", condition, previous)
	}

	if condition.Status != corev1.ConditionTrue || !condition.LastTransitionTime.Equal(&previous.LastTransitionTime) {
		t.Errorf("Update(same failure) condition = %v, want stable failure timestamp %v", condition, previous.LastTransitionTime)
	}

	var recovered namespaceConditionUpdater
	if !recovered.Update(namespace) {
		t.Error("Update(recovered) = false, want failure cleared")
	}

	condition = getCondition(namespace.Status.Conditions, corev1.NamespaceDeletionContentFailure)
	if condition == nil || condition.Status != corev1.ConditionFalse || condition.Reason != "ContentDeleted" {
		t.Errorf("Update(recovered) condition = %v, want False/ContentDeleted", condition)
	}

	custom := getCondition(namespace.Status.Conditions, "Custom")
	if custom == nil || custom.Reason != "Keep" || custom.Status != corev1.ConditionTrue {
		t.Errorf("Update(recovered) custom condition = %v, want unchanged Custom/True/Keep", custom)
	}
}

func TestConditionsRecognizeWrappedDiscoveryFailure(t *testing.T) {
	discoveryErr := &discovery.ErrGroupDiscoveryFailed{
		Groups: map[schema.GroupVersion]error{
			{Group: "example.com", Version: "v1"}: errors.New("discovery unavailable"),
		},
	}
	wrappedErr := fmt.Errorf("refresh discovery: %w", discoveryErr)
	var updater namespaceConditionUpdater
	updater.ProcessDiscoverResourcesErr(wrappedErr)

	condition := getCondition(updater.newConditions, corev1.NamespaceDeletionDiscoveryFailure)
	if condition == nil {
		t.Fatal("ProcessDiscoverResourcesErr(wrapped error) condition = nil, want discovery failure")
	}

	if condition.Status != corev1.ConditionTrue || condition.Reason != "DiscoveryFailed" {
		t.Errorf("ProcessDiscoverResourcesErr(wrapped error) condition = %v, want True/DiscoveryFailed", condition)
	}

	// Message 是 Namespace 状态协议；包装后仍须使用官方的失败组数量说明。
	wantMessage := "Discovery failed for some groups, 1 failing: " + wrappedErr.Error()
	if condition.Message != wantMessage {
		t.Errorf("ProcessDiscoverResourcesErr(wrapped error) Message = %q, want %q", condition.Message, wantMessage)
	}
}
