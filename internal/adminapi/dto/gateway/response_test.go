package gateway

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestGatewayFromResourceShowsDisabledOnlyAfterConfigurationApplied(t *testing.T) {
	const generation int64 = 3
	tests := []struct {
		name       string
		conditions []metav1.Condition
		want       admindto.ResourceStatus
	}{
		{
			name: "active disabled configuration",
			conditions: []metav1.Condition{
				gatewayCondition(metav1.ConditionTrue, resource.ReasonProgrammed, generation),
			},
			want: admindto.NewDisabledResourceStatus(),
		},
		{
			name: "explicitly not applied",
			conditions: []metav1.Condition{
				gatewayCondition(metav1.ConditionFalse, resource.ReasonNotApplied, generation),
			},
			want: admindto.NewDisabledResourceStatus(),
		},
		{
			name: "rejected disabled configuration",
			conditions: []metav1.Condition{
				gatewayCondition(metav1.ConditionFalse, resource.ReasonRejected, generation),
			},
			want: admindto.NewResourceStatus(generation, []metav1.Condition{
				gatewayCondition(metav1.ConditionFalse, resource.ReasonRejected, generation),
			}),
		},
		{
			name: "stale active configuration",
			conditions: []metav1.Condition{
				gatewayCondition(metav1.ConditionTrue, resource.ReasonProgrammed, generation-1),
			},
			want: admindto.NewResourceStatus(generation, []metav1.Condition{
				gatewayCondition(metav1.ConditionTrue, resource.ReasonProgrammed, generation-1),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &resource.Gateway{
				ObjectMeta: metav1.ObjectMeta{Generation: generation},
				Spec:       resource.GatewaySpec{Enabled: false},
				Status:     resource.ResourceStatus{Conditions: tt.conditions},
			}

			got := gatewayFromResource(gateway).Status
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("gatewayFromResource(disabled, generation=%d) status mismatch (-want +got):\n%s", generation, diff)
			}
		})
	}
}

func gatewayCondition(status metav1.ConditionStatus, reason resource.ConditionReason, generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(resource.ConditionProgrammed),
		Status:             status,
		ObservedGeneration: generation,
		Reason:             string(reason),
	}
}
