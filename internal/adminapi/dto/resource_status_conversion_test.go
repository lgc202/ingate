package dto_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	accesscontrolpolicydto "github.com/lgc202/ingate/internal/adminapi/dto/accesscontrolpolicy"
	gatewaydto "github.com/lgc202/ingate/internal/adminapi/dto/gateway"
	policybindingdto "github.com/lgc202/ingate/internal/adminapi/dto/policybinding"
	ratelimitpolicydto "github.com/lgc202/ingate/internal/adminapi/dto/ratelimitpolicy"
	routedto "github.com/lgc202/ingate/internal/adminapi/dto/route"
	accesscontrolpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/accesscontrolpolicy"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	policybindingservice "github.com/lgc202/ingate/internal/adminapi/service/policybinding"
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
					Status:     readyResourceStatus(),
				}
				return ratelimitpolicydto.NewGetRateLimitPolicyResp(&ratelimitpolicyservice.PolicyResult{Policy: policy}).Status
			},
		},
		{
			name: "access control policy",
			status: func(enabled bool) admindto.ResourceStatus {
				policy := &resource.AccessControlPolicy{
					ObjectMeta: resourceMetadata(),
					Spec:       resource.AccessControlPolicySpec{Enabled: enabled},
					Status:     readyResourceStatus(),
				}
				return accesscontrolpolicydto.NewGetAccessControlPolicyResp(&accesscontrolpolicyservice.PolicyResult{Policy: policy}).Status
			},
		},
		{
			name: "policy binding",
			status: func(enabled bool) admindto.ResourceStatus {
				binding := &resource.PolicyBinding{
					ObjectMeta: resourceMetadata(),
					Spec:       resource.PolicyBindingSpec{Enabled: enabled},
					Status:     readyResourceStatus(),
				}
				return policybindingdto.NewGetPolicyBindingResp(&policybindingservice.BindingResult{Binding: binding}).Status
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
