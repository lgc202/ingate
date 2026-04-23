package validation

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestValidateResolvedGatewayRequiresGatewayRefName(t *testing.T) {
	resolvedGateway := &gatewayv1alpha1.ResolvedGateway{}

	errs := ValidateResolvedGateway(resolvedGateway)
	if !hasFieldError(errs, "spec.gatewayRef.name", field.ErrorTypeRequired) {
		t.Fatalf("expected required error for spec.gatewayRef.name, got %v", errs)
	}
}
