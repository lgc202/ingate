package status

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/lgc202/ingate/internal/envoy/config"
	"github.com/lgc202/ingate/internal/envoy/delivery"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestWriterSeparatesReferenceErrorsAndPreservesFutureConditions(t *testing.T) {
	route := &gatewayv1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "route-1",
			UID:        types.UID("route-uid"),
			Generation: 3,
		},
		Status: gatewayv1.ResourceStatus{
			Conditions: []metav1.Condition{{
				Type:               "FutureCondition",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 3,
				Reason:             "FutureReason",
				Message:            "future condition",
			}},
		},
	}
	client := clientfake.NewSimpleClientset(route)
	writer := NewWriter(client)
	diagnostic := config.Diagnostic{
		Severity: config.SeverityError,
		Kind:     gatewayv1.KindRoute,
		ID:       route.Name,
		Reason:   config.ReasonReferenceNotFound,
		Message:  "route references a missing upstream",
	}

	err := writer.ApplyCompileResult(
		context.Background(),
		config.ResourceSet{Routes: []*gatewayv1.Route{route}},
		[]config.Diagnostic{diagnostic},
		delivery.Status{},
	)
	if err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", route.Name, err)
	}
	updated, err := client.GatewayV1().Routes().Get(context.Background(), route.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Routes.Get(%q) error = %v", route.Name, err)
	}

	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionAccepted, metav1.ConditionTrue, gatewayv1.ReasonAccepted, route.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionResolvedRefs, metav1.ConditionFalse, gatewayv1.ReasonReferenceNotFound, route.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionFalse, gatewayv1.ReasonReferenceNotFound, route.Generation)
	if condition := meta.FindStatusCondition(updated.Status.Conditions, "FutureCondition"); condition == nil {
		t.Errorf("Writer.ApplyCompileResult(%q) removed FutureCondition", route.Name)
	}
}

func TestWriterMarksExactActiveResourceProgrammed(t *testing.T) {
	certificate := &gatewayv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "certificate-1",
			UID:        types.UID("certificate-uid"),
			Generation: 2,
		},
	}
	client := clientfake.NewSimpleClientset(certificate)
	writer := NewWriter(client)
	resource := config.ResourceSet{Certificates: []*gatewayv1.Certificate{certificate}}
	active := resource.Generations()[0]

	if err := writer.ApplyCompileResult(context.Background(), resource, nil, delivery.Status{
		ActiveResources: []config.ResourceGeneration{active},
	}); err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", certificate.Name, err)
	}
	updated, err := client.GatewayV1().Certificates().Get(context.Background(), certificate.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Certificates.Get(%q) error = %v", certificate.Name, err)
	}

	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionAccepted, metav1.ConditionTrue, gatewayv1.ReasonAccepted, certificate.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionTrue, gatewayv1.ReasonProgrammed, certificate.Generation)
}

func TestWriterMarksAcceptedResourcePendingWithoutActiveProvenance(t *testing.T) {
	upstream := &gatewayv1.Upstream{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "upstream-1",
			UID:        types.UID("upstream-uid"),
			Generation: 4,
		},
	}
	client := clientfake.NewSimpleClientset(upstream)
	writer := NewWriter(client)

	if err := writer.ApplyCompileResult(
		context.Background(),
		config.ResourceSet{Upstreams: []*gatewayv1.Upstream{upstream}},
		nil,
		delivery.Status{},
	); err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", upstream.Name, err)
	}
	updated, err := client.GatewayV1().Upstreams().Get(context.Background(), upstream.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Upstreams.Get(%q) error = %v", upstream.Name, err)
	}

	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionUnknown, gatewayv1.ReasonPending, upstream.Generation)
}

func TestWriterMarksRejectedCandidateProgrammedFalse(t *testing.T) {
	upstream := &gatewayv1.Upstream{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "upstream-1",
			UID:        types.UID("upstream-uid"),
			Generation: 4,
		},
	}
	client := clientfake.NewSimpleClientset(upstream)
	writer := NewWriter(client)
	resource := config.ResourceSet{Upstreams: []*gatewayv1.Upstream{upstream}}
	failed := resource.Generations()[0]

	if err := writer.ApplyCompileResult(context.Background(), resource, nil, delivery.Status{
		LastFailure: &delivery.Failure{
			Reason:    delivery.FailureRejected,
			Resources: []config.ResourceGeneration{failed},
		},
	}); err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", upstream.Name, err)
	}
	updated, err := client.GatewayV1().Upstreams().Get(context.Background(), upstream.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Upstreams.Get(%q) error = %v", upstream.Name, err)
	}

	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionFalse, gatewayv1.ReasonRejected, upstream.Generation)
}

func TestWriterApplyProgrammedPromotesPendingResourceWithoutChangingCompileConditions(t *testing.T) {
	upstream := &gatewayv1.Upstream{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "upstream-1",
			UID:        types.UID("upstream-uid"),
			Generation: 4,
		},
		Status: gatewayv1.ResourceStatus{
			Conditions: []metav1.Condition{
				{
					Type:               string(gatewayv1.ConditionAccepted),
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 4,
					Reason:             string(gatewayv1.ReasonAccepted),
					Message:            messageAccepted,
				},
				{
					Type:               string(gatewayv1.ConditionProgrammed),
					Status:             metav1.ConditionUnknown,
					ObservedGeneration: 4,
					Reason:             string(gatewayv1.ReasonPending),
					Message:            messagePending,
				},
			},
		},
	}
	client := clientfake.NewSimpleClientset(upstream)
	writer := NewWriter(client)
	resource := config.ResourceSet{Upstreams: []*gatewayv1.Upstream{upstream}}

	if err := writer.ApplyProgrammed(context.Background(), resource, delivery.Status{
		ActiveResources: resource.Generations(),
	}); err != nil {
		t.Fatalf("Writer.ApplyProgrammed(%q) error = %v", upstream.Name, err)
	}
	updated, err := client.GatewayV1().Upstreams().Get(context.Background(), upstream.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Upstreams.Get(%q) error = %v", upstream.Name, err)
	}

	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionAccepted, metav1.ConditionTrue, gatewayv1.ReasonAccepted, upstream.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionTrue, gatewayv1.ReasonProgrammed, upstream.Generation)
}

func TestWriterSkipsDeletedAndRecreatedResourceWithSameName(t *testing.T) {
	compiled := &gatewayv1.Upstream{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "upstream-1",
			UID:        types.UID("old-uid"),
			Generation: 1,
		},
	}
	recreated := compiled.DeepCopy()
	recreated.UID = types.UID("new-uid")
	client := clientfake.NewSimpleClientset(recreated)
	writer := NewWriter(client)

	if err := writer.ApplyCompileResult(
		context.Background(),
		config.ResourceSet{Upstreams: []*gatewayv1.Upstream{compiled}},
		nil,
		delivery.Status{},
	); err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", compiled.Name, err)
	}
	updated, err := client.GatewayV1().Upstreams().Get(context.Background(), recreated.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Upstreams.Get(%q) error = %v", recreated.Name, err)
	}
	if len(updated.Status.Conditions) != 0 {
		t.Errorf("Writer.ApplyCompileResult(%q) conditions = %v, want none for recreated UID", compiled.Name, updated.Status.Conditions)
	}
}

func assertCondition(
	t *testing.T,
	conditions []metav1.Condition,
	conditionType gatewayv1.ConditionType,
	wantStatus metav1.ConditionStatus,
	wantReason gatewayv1.ConditionReason,
	wantGeneration int64,
) {
	t.Helper()
	condition := meta.FindStatusCondition(conditions, string(conditionType))
	if condition == nil {
		t.Fatalf("FindStatusCondition(%q) = nil, want condition", conditionType)
	}
	if condition.Status != wantStatus {
		t.Errorf("condition %q status = %q, want %q", conditionType, condition.Status, wantStatus)
	}
	if condition.Reason != string(wantReason) {
		t.Errorf("condition %q reason = %q, want %q", conditionType, condition.Reason, wantReason)
	}
	if condition.ObservedGeneration != wantGeneration {
		t.Errorf("condition %q observed generation = %d, want %d", conditionType, condition.ObservedGeneration, wantGeneration)
	}
}
