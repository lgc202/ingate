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

func TestWriterReportsMissingUpstreamCredentialAsUnresolvedReference(t *testing.T) {
	upstream := &gatewayv1.Upstream{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "upstream-1",
			UID:        types.UID("upstream-uid"),
			Generation: 2,
		},
	}
	client := clientfake.NewSimpleClientset(upstream)
	writer := NewWriter(client)

	if err := writer.ApplyCompileResult(
		context.Background(),
		config.ResourceSet{Upstreams: []*gatewayv1.Upstream{upstream}},
		[]config.Diagnostic{{
			Severity: config.SeverityError,
			Kind:     gatewayv1.KindUpstream,
			ID:       upstream.Name,
			Reason:   config.ReasonReferenceNotFound,
			Message:  "credential not found",
		}},
		delivery.Status{},
	); err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", upstream.Name, err)
	}
	updated, err := client.GatewayV1().Upstreams().Get(context.Background(), upstream.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Upstreams.Get(%q) error = %v", upstream.Name, err)
	}

	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionAccepted, metav1.ConditionTrue, gatewayv1.ReasonAccepted, upstream.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionResolvedRefs, metav1.ConditionFalse, gatewayv1.ReasonReferenceNotFound, upstream.Generation)
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
					Type:               string(gatewayv1.ConditionResolvedRefs),
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 4,
					Reason:             string(gatewayv1.ReasonResolvedRefs),
					Message:            messageResolvedRefs,
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
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionResolvedRefs, metav1.ConditionTrue, gatewayv1.ReasonResolvedRefs, upstream.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionTrue, gatewayv1.ReasonProgrammed, upstream.Generation)
}

func TestWriterKeepsValidPolicyTargetProgrammedWhenAnotherTargetIsMissing(t *testing.T) {
	gateway := &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"}}
	policy := &gatewayv1.RateLimitPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "rate-limit-1",
			UID:        types.UID("rate-limit-uid"),
			Generation: 2,
		},
		Spec: gatewayv1.RateLimitPolicySpec{
			TargetRefs: []gatewayv1.PolicyTargetRef{
				{Kind: gatewayv1.KindGateway, Name: gateway.Name},
				{Kind: gatewayv1.KindRoute, Name: "missing-route"},
			},
		},
	}
	client := clientfake.NewSimpleClientset(gateway, policy)
	writer := NewWriter(client)
	resources := config.ResourceSet{
		Gateways:          []*gatewayv1.Gateway{gateway},
		RateLimitPolicies: []*gatewayv1.RateLimitPolicy{policy},
	}
	diagnostic := config.Diagnostic{
		Severity: config.SeverityWarning,
		Kind:     gatewayv1.KindRateLimitPolicy,
		ID:       policy.Name,
		Reason:   config.ReasonReferenceNotFound,
		Message:  "policy references a missing route",
	}

	if err := writer.ApplyCompileResult(
		context.Background(),
		resources,
		[]config.Diagnostic{diagnostic},
		delivery.Status{
			ActiveResources: resources.Generations(),
			ActivePolicyTargets: []config.ProgrammedPolicyTarget{
				{
					Policy: config.ResourceGeneration{
						Kind:       gatewayv1.KindRateLimitPolicy,
						Name:       policy.Name,
						UID:        policy.UID,
						Generation: policy.Generation,
					},
					Target: config.ResourceGeneration{
						Kind:       gatewayv1.KindGateway,
						Name:       gateway.Name,
						UID:        gateway.UID,
						Generation: gateway.Generation,
					},
				},
			},
		},
	); err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", policy.Name, err)
	}
	updated, err := client.GatewayV1().RateLimitPolicies().Get(context.Background(), policy.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("RateLimitPolicies.Get(%q) error = %v", policy.Name, err)
	}

	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionAccepted, metav1.ConditionTrue, gatewayv1.ReasonAccepted, policy.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionResolvedRefs, metav1.ConditionFalse, gatewayv1.ReasonReferenceNotFound, policy.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionTrue, gatewayv1.ReasonProgrammed, policy.Generation)
	validTarget := findPolicyTargetStatus(t, updated.Status.Targets, gatewayv1.PolicyTargetRef{Kind: gatewayv1.KindGateway, Name: gateway.Name})
	assertCondition(t, validTarget.Conditions, gatewayv1.ConditionResolvedRefs, metav1.ConditionTrue, gatewayv1.ReasonResolvedRefs, policy.Generation)
	assertCondition(t, validTarget.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionTrue, gatewayv1.ReasonProgrammed, policy.Generation)
	missingTarget := findPolicyTargetStatus(t, updated.Status.Targets, gatewayv1.PolicyTargetRef{Kind: gatewayv1.KindRoute, Name: "missing-route"})
	assertCondition(t, missingTarget.Conditions, gatewayv1.ConditionResolvedRefs, metav1.ConditionFalse, gatewayv1.ReasonReferenceNotFound, policy.Generation)
	assertCondition(t, missingTarget.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionFalse, gatewayv1.ReasonReferenceNotFound, policy.Generation)
}

func TestWriterMarksPolicyWithoutTargetsNotApplied(t *testing.T) {
	policy := &gatewayv1.AccessControlPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "access-control-1",
			UID:        types.UID("access-control-uid"),
			Generation: 1,
		},
	}
	client := clientfake.NewSimpleClientset(policy)
	writer := NewWriter(client)
	resources := config.ResourceSet{AccessControlPolicies: []*gatewayv1.AccessControlPolicy{policy}}

	if err := writer.ApplyCompileResult(
		context.Background(),
		resources,
		nil,
		delivery.Status{ActiveResources: resources.Generations()},
	); err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", policy.Name, err)
	}
	updated, err := client.GatewayV1().AccessControlPolicies().Get(context.Background(), policy.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("AccessControlPolicies.Get(%q) error = %v", policy.Name, err)
	}

	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionAccepted, metav1.ConditionTrue, gatewayv1.ReasonAccepted, policy.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionResolvedRefs, metav1.ConditionTrue, gatewayv1.ReasonResolvedRefs, policy.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionFalse, gatewayv1.ReasonNotApplied, policy.Generation)
	if len(updated.Status.Targets) != 0 {
		t.Fatalf("len(Targets) = %d, want 0", len(updated.Status.Targets))
	}
}

func TestWriterKeepsPolicyWithoutTargetsPendingOrFailedUntilRemovalIsActive(t *testing.T) {
	tests := []struct {
		name       string
		status     func(config.ResourceSet) delivery.Status
		wantStatus metav1.ConditionStatus
		wantReason gatewayv1.ConditionReason
	}{
		{
			name:       "pending",
			status:     func(config.ResourceSet) delivery.Status { return delivery.Status{} },
			wantStatus: metav1.ConditionUnknown,
			wantReason: gatewayv1.ReasonPending,
		},
		{
			name: "rejected",
			status: func(resources config.ResourceSet) delivery.Status {
				return delivery.Status{LastFailure: &delivery.Failure{
					Reason:    delivery.FailureRejected,
					Resources: resources.Generations(),
				}}
			},
			wantStatus: metav1.ConditionFalse,
			wantReason: gatewayv1.ReasonRejected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &gatewayv1.RateLimitPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "rate-limit-1",
					UID:        types.UID("rate-limit-uid"),
					Generation: 2,
				},
			}
			client := clientfake.NewSimpleClientset(policy)
			writer := NewWriter(client)
			resources := config.ResourceSet{RateLimitPolicies: []*gatewayv1.RateLimitPolicy{policy}}

			if err := writer.ApplyCompileResult(context.Background(), resources, nil, tt.status(resources)); err != nil {
				t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", policy.Name, err)
			}
			updated, err := client.GatewayV1().RateLimitPolicies().Get(context.Background(), policy.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("RateLimitPolicies.Get(%q) error = %v", policy.Name, err)
			}

			assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, tt.wantStatus, tt.wantReason, policy.Generation)
		})
	}
}

func TestWriterMarksExistingButUnattachedPolicyTargetNotApplied(t *testing.T) {
	gateway := &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"}}
	policy := &gatewayv1.AccessControlPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "access-control-1",
			UID:        types.UID("access-control-uid"),
			Generation: 1,
		},
		Spec: gatewayv1.AccessControlPolicySpec{
			TargetRefs: []gatewayv1.PolicyTargetRef{{Kind: gatewayv1.KindGateway, Name: gateway.Name}},
		},
	}
	client := clientfake.NewSimpleClientset(gateway, policy)
	writer := NewWriter(client)
	resources := config.ResourceSet{
		Gateways:              []*gatewayv1.Gateway{gateway},
		AccessControlPolicies: []*gatewayv1.AccessControlPolicy{policy},
	}

	if err := writer.ApplyCompileResult(
		context.Background(),
		resources,
		nil,
		delivery.Status{ActiveResources: resources.Generations()},
	); err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", policy.Name, err)
	}
	updated, err := client.GatewayV1().AccessControlPolicies().Get(context.Background(), policy.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("AccessControlPolicies.Get(%q) error = %v", policy.Name, err)
	}

	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionFalse, gatewayv1.ReasonNotApplied, policy.Generation)
	target := findPolicyTargetStatus(t, updated.Status.Targets, policy.Spec.TargetRefs[0])
	assertCondition(t, target.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionFalse, gatewayv1.ReasonNotApplied, policy.Generation)
}

func TestWriterMarksPolicyPartiallyAppliedWhenOneTargetIsNotProgrammed(t *testing.T) {
	gateway := &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", UID: types.UID("gateway-uid"), Generation: 1}}
	route := &gatewayv1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route-1", UID: types.UID("route-uid"), Generation: 1}}
	policy := &gatewayv1.RateLimitPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "rate-limit-1", UID: types.UID("policy-uid"), Generation: 1},
		Spec: gatewayv1.RateLimitPolicySpec{TargetRefs: []gatewayv1.PolicyTargetRef{
			{Kind: gatewayv1.KindGateway, Name: gateway.Name},
			{Kind: gatewayv1.KindRoute, Name: route.Name},
		}},
	}
	client := clientfake.NewSimpleClientset(gateway, route, policy)
	writer := NewWriter(client)
	resources := config.ResourceSet{
		Gateways:          []*gatewayv1.Gateway{gateway},
		Routes:            []*gatewayv1.Route{route},
		RateLimitPolicies: []*gatewayv1.RateLimitPolicy{policy},
	}
	policyGeneration := config.ResourceGeneration{Kind: gatewayv1.KindRateLimitPolicy, Name: policy.Name, UID: policy.UID, Generation: policy.Generation}
	gatewayGeneration := config.ResourceGeneration{Kind: gatewayv1.KindGateway, Name: gateway.Name, UID: gateway.UID, Generation: gateway.Generation}

	if err := writer.ApplyCompileResult(context.Background(), resources, nil, delivery.Status{
		ActiveResources: resources.Generations(),
		ActivePolicyTargets: []config.ProgrammedPolicyTarget{{
			Policy: policyGeneration,
			Target: gatewayGeneration,
		}},
	}); err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", policy.Name, err)
	}
	updated, err := client.GatewayV1().RateLimitPolicies().Get(context.Background(), policy.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("RateLimitPolicies.Get(%q) error = %v", policy.Name, err)
	}

	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionTrue, gatewayv1.ReasonProgrammed, policy.Generation)
	programmed := findPolicyTargetStatus(t, updated.Status.Targets, policy.Spec.TargetRefs[0])
	assertCondition(t, programmed.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionTrue, gatewayv1.ReasonProgrammed, policy.Generation)
	notApplied := findPolicyTargetStatus(t, updated.Status.Targets, policy.Spec.TargetRefs[1])
	assertCondition(t, notApplied.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionFalse, gatewayv1.ReasonNotApplied, policy.Generation)
}

func TestWriterDoesNotReuseProgrammedTargetAcrossTargetGenerations(t *testing.T) {
	route := &gatewayv1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route-1", UID: types.UID("route-uid"), Generation: 2}}
	policy := &gatewayv1.AccessControlPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "access-control-1", UID: types.UID("policy-uid"), Generation: 1},
		Spec: gatewayv1.AccessControlPolicySpec{
			TargetRefs: []gatewayv1.PolicyTargetRef{{Kind: gatewayv1.KindRoute, Name: route.Name}},
		},
	}
	client := clientfake.NewSimpleClientset(route, policy)
	writer := NewWriter(client)
	resources := config.ResourceSet{
		Routes:                []*gatewayv1.Route{route},
		AccessControlPolicies: []*gatewayv1.AccessControlPolicy{policy},
	}
	policyGeneration := config.ResourceGeneration{Kind: gatewayv1.KindAccessControlPolicy, Name: policy.Name, UID: policy.UID, Generation: policy.Generation}
	oldRouteGeneration := config.ResourceGeneration{Kind: gatewayv1.KindRoute, Name: route.Name, UID: route.UID, Generation: 1}

	if err := writer.ApplyCompileResult(context.Background(), resources, nil, delivery.Status{
		ActiveResources: []config.ResourceGeneration{policyGeneration, oldRouteGeneration},
		ActivePolicyTargets: []config.ProgrammedPolicyTarget{{
			Policy: policyGeneration,
			Target: oldRouteGeneration,
		}},
	}); err != nil {
		t.Fatalf("Writer.ApplyCompileResult(%q) error = %v", policy.Name, err)
	}
	updated, err := client.GatewayV1().AccessControlPolicies().Get(context.Background(), policy.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("AccessControlPolicies.Get(%q) error = %v", policy.Name, err)
	}

	target := findPolicyTargetStatus(t, updated.Status.Targets, policy.Spec.TargetRefs[0])
	assertCondition(t, target.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionUnknown, gatewayv1.ReasonPending, policy.Generation)
	assertCondition(t, updated.Status.Conditions, gatewayv1.ConditionProgrammed, metav1.ConditionUnknown, gatewayv1.ReasonPending, policy.Generation)
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

func findPolicyTargetStatus(
	t *testing.T,
	statuses []gatewayv1.PolicyTargetStatus,
	target gatewayv1.PolicyTargetRef,
) gatewayv1.PolicyTargetStatus {
	t.Helper()
	for _, status := range statuses {
		if status.TargetRef == target {
			return status
		}
	}
	t.Fatalf("policy target status %v not found", target)
	return gatewayv1.PolicyTargetStatus{}
}
