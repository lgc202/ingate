package dto_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	accesscontrolpolicydto "github.com/lgc202/ingate/internal/adminapi/dto/accesscontrolpolicy"
	gatewaydto "github.com/lgc202/ingate/internal/adminapi/dto/gateway"
	ratelimitpolicydto "github.com/lgc202/ingate/internal/adminapi/dto/ratelimitpolicy"
	routedto "github.com/lgc202/ingate/internal/adminapi/dto/route"
	accesscontrolpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/accesscontrolpolicy"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	ratelimitpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimitpolicy"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const resourceGeneration int64 = 1

func TestEnabledResourceStatusConversion(t *testing.T) {
	tests := []struct {
		name   string
		status func(bool) admindto.ResourceStatus
	}{
		{
			name: "gateway",
			status: func(enabled bool) admindto.ResourceStatus {
				gateway := &resource.Gateway{
					ObjectMeta: resourceMetadata(),
					Spec:       resource.GatewaySpec{Enabled: enabled},
					Status:     readyResourceStatus(),
				}
				return gatewaydto.NewGetGatewayResp(&gatewayservice.GatewayResult{Gateway: gateway}).Gateway.Status
			},
		},
		{
			name: "route",
			status: func(enabled bool) admindto.ResourceStatus {
				route := &resource.Route{
					ObjectMeta: resourceMetadata(),
					Spec:       resource.RouteSpec{Enabled: enabled},
					Status:     readyResourceStatus(),
				}
				return routedto.NewGetRouteResp(&routeservice.RouteResult{Route: route}).Status
			},
		},
		{
			name: "rate limit policy",
			status: func(enabled bool) admindto.ResourceStatus {
				policy := &resource.RateLimitPolicy{
					ObjectMeta: resourceMetadata(),
					Spec:       resource.RateLimitPolicySpec{Enabled: enabled},
					Status:     readyPolicyStatus(),
				}
				status := ratelimitpolicydto.NewGetRateLimitPolicyResp(&ratelimitpolicyservice.PolicyResult{Policy: policy}).Status
				return admindto.ResourceStatus{State: status.State, Message: status.Message}
			},
		},
		{
			name: "access control policy",
			status: func(enabled bool) admindto.ResourceStatus {
				policy := &resource.AccessControlPolicy{
					ObjectMeta: resourceMetadata(),
					Spec:       resource.AccessControlPolicySpec{Enabled: enabled},
					Status:     readyPolicyStatus(),
				}
				status := accesscontrolpolicydto.NewGetAccessControlPolicyResp(&accesscontrolpolicyservice.PolicyResult{Policy: policy}).Status
				return admindto.ResourceStatus{State: status.State, Message: status.Message}
			},
		},
	}
	states := []struct {
		name    string
		enabled bool
		want    admindto.ResourceStatus
	}{
		{
			name:    "enabled",
			enabled: true,
			want:    admindto.ResourceStatus{State: admindto.ResourceStateReady, Message: "配置已生效"},
		},
		{
			name:    "disabled",
			enabled: false,
			want:    admindto.ResourceStatus{State: admindto.ResourceStateDisabled, Message: "已停用"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, state := range states {
				t.Run(state.name, func(t *testing.T) {
					got := tt.status(state.enabled)
					if diff := cmp.Diff(state.want, got); diff != "" {
						t.Errorf("DTO status conversion(resource=%q, enabled=%t) mismatch (-want +got):\n%s", tt.name, state.enabled, diff)
					}
				})
			}
		})
	}
}

func TestPolicyTargetStatusConversion(t *testing.T) {
	targetRef := resource.PolicyTargetRef{Kind: resource.KindGateway, Name: "gateway-1"}
	targetNames := policytarget.DisplayNames{
		{Kind: resource.KindGateway, ID: targetRef.Name}: "生产网关",
	}
	want := admindto.PolicyTarget{
		Kind:        admindto.PolicyTargetKindGateway,
		ID:          targetRef.Name,
		DisplayName: "生产网关",
		Status:      admindto.ResourceStatus{State: admindto.ResourceStateReady, Message: "配置已生效"},
	}
	tests := []struct {
		name   string
		target func() admindto.PolicyTarget
	}{
		{
			name: "rate limit policy",
			target: func() admindto.PolicyTarget {
				policy := &resource.RateLimitPolicy{
					ObjectMeta: resourceMetadata(),
					Spec: resource.RateLimitPolicySpec{
						Enabled:    true,
						TargetRefs: []resource.PolicyTargetRef{targetRef},
					},
					Status: resource.PolicyStatus{
						Targets: []resource.PolicyTargetStatus{{TargetRef: targetRef, Conditions: readyPolicyTargetConditions()}},
					},
				}
				return ratelimitpolicydto.NewGetRateLimitPolicyResp(&ratelimitpolicyservice.PolicyResult{Policy: policy, TargetNames: targetNames}).Targets[0]
			},
		},
		{
			name: "access control policy",
			target: func() admindto.PolicyTarget {
				policy := &resource.AccessControlPolicy{
					ObjectMeta: resourceMetadata(),
					Spec: resource.AccessControlPolicySpec{
						Enabled:    true,
						TargetRefs: []resource.PolicyTargetRef{targetRef},
					},
					Status: resource.PolicyStatus{
						Targets: []resource.PolicyTargetStatus{{TargetRef: targetRef, Conditions: readyPolicyTargetConditions()}},
					},
				}
				return accesscontrolpolicydto.NewGetAccessControlPolicyResp(&accesscontrolpolicyservice.PolicyResult{Policy: policy, TargetNames: targetNames}).Targets[0]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(want, tt.target()); diff != "" {
				t.Errorf("policy target conversion mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func resourceMetadata() metav1.ObjectMeta {
	return metav1.ObjectMeta{Generation: resourceGeneration}
}

func readyResourceStatus() resource.ResourceStatus {
	return resource.ResourceStatus{
		Conditions: []metav1.Condition{
			{
				Type:               string(resource.ConditionAccepted),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: resourceGeneration,
				Reason:             string(resource.ReasonAccepted),
			},
			{
				Type:               string(resource.ConditionProgrammed),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: resourceGeneration,
				Reason:             string(resource.ReasonProgrammed),
			},
		},
	}
}

func readyPolicyStatus() resource.PolicyStatus {
	return resource.PolicyStatus{Conditions: readyResourceStatus().Conditions}
}

func readyPolicyTargetConditions() []metav1.Condition {
	return []metav1.Condition{
		{
			Type:               string(resource.ConditionResolvedRefs),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: resourceGeneration,
			Reason:             string(resource.ReasonResolvedRefs),
		},
		{
			Type:               string(resource.ConditionProgrammed),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: resourceGeneration,
			Reason:             string(resource.ReasonProgrammed),
		},
	}
}
