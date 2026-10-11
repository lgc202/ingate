package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateStatusUpdate(t *testing.T) {
	tests := []struct {
		name     string
		validate func(metav1.Condition) field.ErrorList
	}{
		{
			name: "Gateway",
			validate: func(condition metav1.Condition) field.ErrorList {
				previous := &gatewayapi.Gateway{ObjectMeta: metav1.ObjectMeta{
					Name: "edge", ResourceVersion: "1", Generation: 2,
				}}
				next := previous.DeepCopy()
				next.Status.Conditions = []metav1.Condition{condition}
				return ValidateGatewayStatusUpdate(next, previous)
			},
		},
		{
			name: "HTTPRoute",
			validate: func(condition metav1.Condition) field.ErrorList {
				previous := &gatewayapi.HTTPRoute{ObjectMeta: metav1.ObjectMeta{
					Name: "api", ResourceVersion: "1", Generation: 2,
				}}
				next := previous.DeepCopy()
				next.Status.Parents = []gatewayapi.RouteParentStatus{{
					ParentRef:      gatewayapi.ParentReference{Name: "edge"},
					ControllerName: "gateway.ingate.io/test",
					Conditions:     []metav1.Condition{condition},
				}}
				return ValidateHTTPRouteStatusUpdate(next, previous)
			},
		},
		{
			name: "Backend",
			validate: func(condition metav1.Condition) field.ErrorList {
				previous := &gatewayapi.Backend{ObjectMeta: metav1.ObjectMeta{
					Name: "backend", ResourceVersion: "1", Generation: 2,
				}}
				next := previous.DeepCopy()
				next.Status.Conditions = []metav1.Condition{condition}
				return ValidateBackendStatusUpdate(next, previous)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition := metav1.Condition{
				Type: "Accepted", Status: metav1.ConditionTrue,
				ObservedGeneration: 2, LastTransitionTime: metav1.Now(),
				Reason: "Configured", Message: "ready",
			}
			wantField := "status.conditions[0].observedGeneration"
			if tt.name == "HTTPRoute" {
				wantField = "status.parents[0].conditions[0].observedGeneration"
			}
			if errs := tt.validate(condition); len(errs) != 0 {
				t.Errorf("Validate%sStatusUpdate(observedGeneration=2) errors = %v, want none", tt.name, errs)
			}

			condition.ObservedGeneration = 3
			if errs := tt.validate(condition); len(errs) != 1 || errs[0].Field != wantField {
				t.Errorf("Validate%sStatusUpdate(observedGeneration=3) errors = %v, want observedGeneration error", tt.name, errs)
			}
		})
	}
}
