package status

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"

	"github.com/lgc202/ingate/internal/envoy/config"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	fakeclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestAcceptedWriterApplyDiagnostics(t *testing.T) {
	ctx := context.Background()
	const generation int64 = 7

	gateway := &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-a", Generation: generation}}
	route := &gatewayv1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route-a", Generation: generation}}
	upstream := &gatewayv1.Upstream{ObjectMeta: metav1.ObjectMeta{Name: "upstream-a", Generation: generation}}
	rateLimitPolicy := &gatewayv1.RateLimitPolicy{ObjectMeta: metav1.ObjectMeta{Name: "rate-a", Generation: generation}}
	accessControlPolicy := &gatewayv1.AccessControlPolicy{ObjectMeta: metav1.ObjectMeta{Name: "access-a", Generation: generation}}
	policyBinding := &gatewayv1.PolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "binding-a", Generation: generation}}
	client := fakeclient.NewSimpleClientset()
	createStatusTestResources(
		t,
		ctx,
		client,
		gateway,
		route,
		upstream,
		rateLimitPolicy,
		accessControlPolicy,
		policyBinding,
	)
	writer := NewAcceptedWriter(client)
	resources := config.ResourceSet{
		Gateways:              []*gatewayv1.Gateway{gateway},
		Routes:                []*gatewayv1.Route{route},
		Upstreams:             []*gatewayv1.Upstream{upstream},
		RateLimitPolicies:     []*gatewayv1.RateLimitPolicy{rateLimitPolicy},
		AccessControlPolicies: []*gatewayv1.AccessControlPolicy{accessControlPolicy},
		PolicyBindings:        []*gatewayv1.PolicyBinding{policyBinding},
	}
	diagnostics := []config.Diagnostic{
		{
			Severity: config.SeverityError,
			Kind:     gatewayv1.KindRoute,
			ID:       route.Name,
			Reason:   config.ReasonReferenceNotFound,
			Message:  "upstream is missing",
		},
		{
			Severity: config.SeverityWarning,
			Kind:     gatewayv1.KindGateway,
			ID:       gateway.Name,
			Reason:   config.ReasonUnsupported,
			Message:  "warning does not reject the resource",
		},
	}

	if err := writer.ApplyDiagnostics(ctx, resources, diagnostics); err != nil {
		t.Fatalf("ApplyDiagnostics() error = %v", err)
	}

	storedGateway, err := client.GatewayV1().Gateways().Get(ctx, gateway.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get Gateway error = %v", err)
	}
	assertAcceptedCondition(t, storedGateway.Status.Conditions, generation, metav1.ConditionTrue, config.ReasonAccepted)

	storedRoute, err := client.GatewayV1().Routes().Get(ctx, route.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get Route error = %v", err)
	}
	assertAcceptedCondition(t, storedRoute.Status.Conditions, generation, metav1.ConditionFalse, config.ReasonReferenceNotFound)
	if got, want := storedRoute.Status.Conditions[0].Message, "upstream is missing"; got != want {
		t.Errorf("Route Accepted message = %q, want %q", got, want)
	}

	storedUpstream, err := client.GatewayV1().Upstreams().Get(ctx, upstream.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get Upstream error = %v", err)
	}
	assertAcceptedCondition(t, storedUpstream.Status.Conditions, generation, metav1.ConditionTrue, config.ReasonAccepted)

	storedRateLimitPolicy, err := client.GatewayV1().RateLimitPolicies().Get(ctx, rateLimitPolicy.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get RateLimitPolicy error = %v", err)
	}
	assertAcceptedCondition(t, storedRateLimitPolicy.Status.Conditions, generation, metav1.ConditionTrue, config.ReasonAccepted)

	storedAccessControlPolicy, err := client.GatewayV1().AccessControlPolicies().Get(ctx, accessControlPolicy.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get AccessControlPolicy error = %v", err)
	}
	assertAcceptedCondition(t, storedAccessControlPolicy.Status.Conditions, generation, metav1.ConditionTrue, config.ReasonAccepted)

	storedPolicyBinding, err := client.GatewayV1().PolicyBindings().Get(ctx, policyBinding.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get PolicyBinding error = %v", err)
	}
	assertAcceptedCondition(t, storedPolicyBinding.Status.Conditions, generation, metav1.ConditionTrue, config.ReasonAccepted)

	if got, want := statusUpdateCount(client.Actions()), 6; got != want {
		t.Fatalf("status update count = %d, want %d", got, want)
	}
	if err := writer.ApplyDiagnostics(ctx, resources, diagnostics); err != nil {
		t.Fatalf("second ApplyDiagnostics() error = %v", err)
	}
	if got, want := statusUpdateCount(client.Actions()), 6; got != want {
		t.Errorf("status update count after unchanged apply = %d, want %d", got, want)
	}
}

func TestAcceptedWriterRetriesConflict(t *testing.T) {
	ctx := context.Background()
	gateway := &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-a", Generation: 3}}
	client := fakeclient.NewSimpleClientset()
	if _, err := client.GatewayV1().Gateways().Create(ctx, gateway.DeepCopy(), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create Gateway error = %v", err)
	}
	attempts := 0
	client.PrependReactor("update", string(gatewayv1.ResourceGateways), func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		attempts++
		if attempts == 1 {
			return true, nil, apierrors.NewConflict(
				gatewayv1.Resource(gatewayv1.ResourceGateways),
				gateway.Name,
				errors.New("conflict"),
			)
		}
		return false, nil, nil
	})

	writer := NewAcceptedWriter(client)
	if err := writer.ApplyDiagnostics(ctx, config.ResourceSet{Gateways: []*gatewayv1.Gateway{gateway}}, nil); err != nil {
		t.Fatalf("ApplyDiagnostics() error = %v", err)
	}
	if got, want := attempts, 2; got != want {
		t.Errorf("UpdateStatus attempts = %d, want %d", got, want)
	}
}

func TestAcceptedWriterSkipsNewGeneration(t *testing.T) {
	ctx := context.Background()
	compiled := &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-a", Generation: 2}}
	current := compiled.DeepCopy()
	current.Generation = 3
	client := fakeclient.NewSimpleClientset()
	if _, err := client.GatewayV1().Gateways().Create(ctx, current, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create Gateway error = %v", err)
	}

	writer := NewAcceptedWriter(client)
	if err := writer.ApplyDiagnostics(ctx, config.ResourceSet{Gateways: []*gatewayv1.Gateway{compiled}}, nil); err != nil {
		t.Fatalf("ApplyDiagnostics() error = %v", err)
	}
	if got := statusUpdateCount(client.Actions()); got != 0 {
		t.Errorf("status update count = %d, want 0", got)
	}
}

func assertAcceptedCondition(
	t *testing.T,
	conditions []metav1.Condition,
	generation int64,
	conditionStatus metav1.ConditionStatus,
	reason config.Reason,
) {
	t.Helper()
	if got, want := len(conditions), 1; got != want {
		t.Fatalf("Accepted condition count = %d, want %d", got, want)
	}
	condition := conditions[0]
	if got, want := condition.Type, conditionAccepted; got != want {
		t.Errorf("Accepted condition type = %q, want %q", got, want)
	}
	if got, want := condition.Status, conditionStatus; got != want {
		t.Errorf("Accepted condition status = %q, want %q", got, want)
	}
	if got, want := condition.ObservedGeneration, generation; got != want {
		t.Errorf("Accepted condition observed generation = %d, want %d", got, want)
	}
	if got, want := condition.Reason, string(reason); got != want {
		t.Errorf("Accepted condition reason = %q, want %q", got, want)
	}
	if condition.LastTransitionTime.IsZero() {
		t.Error("Accepted condition last transition time is zero")
	}
}

func statusUpdateCount(actions []clienttesting.Action) int {
	count := 0
	for _, action := range actions {
		if action.GetVerb() == "update" && action.GetSubresource() == "status" {
			count++
		}
	}
	return count
}

func createStatusTestResources(
	t *testing.T,
	ctx context.Context,
	client *fakeclient.Clientset,
	gateway *gatewayv1.Gateway,
	route *gatewayv1.Route,
	upstream *gatewayv1.Upstream,
	rateLimitPolicy *gatewayv1.RateLimitPolicy,
	accessControlPolicy *gatewayv1.AccessControlPolicy,
	policyBinding *gatewayv1.PolicyBinding,
) {
	t.Helper()
	if _, err := client.GatewayV1().Gateways().Create(ctx, gateway.DeepCopy(), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create Gateway error = %v", err)
	}
	if _, err := client.GatewayV1().Routes().Create(ctx, route.DeepCopy(), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create Route error = %v", err)
	}
	if _, err := client.GatewayV1().Upstreams().Create(ctx, upstream.DeepCopy(), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create Upstream error = %v", err)
	}
	if _, err := client.GatewayV1().RateLimitPolicies().Create(ctx, rateLimitPolicy.DeepCopy(), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create RateLimitPolicy error = %v", err)
	}
	if _, err := client.GatewayV1().AccessControlPolicies().Create(ctx, accessControlPolicy.DeepCopy(), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create AccessControlPolicy error = %v", err)
	}
	if _, err := client.GatewayV1().PolicyBindings().Create(ctx, policyBinding.DeepCopy(), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create PolicyBinding error = %v", err)
	}
}
