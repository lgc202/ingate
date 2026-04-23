package table

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	commonregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/common"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

var (
	authPolicyColumns = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "AuthPolicy name."},
		{Name: "Type", Type: "string", Description: "Authentication policy type."},
		{Name: "Targets", Type: "integer", Description: "Number of targetRefs."},
		{Name: "Age", Type: "string", Description: "Time since creation."},
	}
	trafficPolicyColumns = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "TrafficPolicy name."},
		{Name: "Targets", Type: "integer", Description: "Number of targetRefs."},
		{Name: "Timeout", Type: "string", Description: "Configured timeout duration."},
		{Name: "RateLimit", Type: "string", Description: "Configured rate limit summary."},
		{Name: "Age", Type: "string", Description: "Time since creation."},
	}
)

func AuthPolicyCells(obj runtime.Object) ([]interface{}, error) {
	policy, ok := obj.(*policyv1alpha1.AuthPolicy)
	if !ok {
		return nil, fmt.Errorf("expected AuthPolicy, got %T", obj)
	}
	return []interface{}{
		policy.Name,
		policy.Spec.Type,
		len(policy.Spec.TargetRefs),
		commonregistry.FormatTimestampAge(policy.CreationTimestamp),
	}, nil
}

func TrafficPolicyCells(obj runtime.Object) ([]interface{}, error) {
	policy, ok := obj.(*policyv1alpha1.TrafficPolicy)
	if !ok {
		return nil, fmt.Errorf("expected TrafficPolicy, got %T", obj)
	}
	return []interface{}{
		policy.Name,
		len(policy.Spec.TargetRefs),
		timeoutValue(policy.Spec.Timeout),
		rateLimitValue(policy.Spec.RateLimit),
		commonregistry.FormatTimestampAge(policy.CreationTimestamp),
	}, nil
}

func AuthPolicyColumns() []metav1.TableColumnDefinition    { return authPolicyColumns }
func TrafficPolicyColumns() []metav1.TableColumnDefinition { return trafficPolicyColumns }

func timeoutValue(timeout *policyv1alpha1.TimeoutSpec) string {
	if timeout == nil || timeout.Duration == "" {
		return "-"
	}
	return timeout.Duration
}

func rateLimitValue(rateLimit *policyv1alpha1.RateLimitSpec) string {
	if rateLimit == nil || rateLimit.RequestsPerUnit == 0 || rateLimit.Unit == "" {
		return "-"
	}
	parts := []string{fmt.Sprintf("%d/%s", rateLimit.RequestsPerUnit, rateLimit.Unit)}
	if rateLimit.Scope != "" {
		parts = append(parts, rateLimit.Scope)
	}
	return strings.Join(parts, " ")
}
