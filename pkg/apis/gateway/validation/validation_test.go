package validation

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestValidateBackendAllowsOmittedProtocolOnCreate(t *testing.T) {
	backend := validBackendFixture()
	backend.Spec.Protocol = ""

	if errs := ValidateBackend(backend); len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %v", errs)
	}
}

func TestValidateBackendRejectsInvalidProtocol(t *testing.T) {
	backend := validBackendFixture()
	backend.Spec.Protocol = "FTP"

	errs := ValidateBackend(backend)
	if !hasFieldError(errs, "spec.protocol", field.ErrorTypeNotSupported) {
		t.Fatalf("expected not-supported error for spec.protocol, got %v", errs)
	}
}

func TestValidateBackendUpdateRequiresProtocol(t *testing.T) {
	backend := validBackendFixture()
	backend.Spec.Protocol = ""

	errs := ValidateBackendUpdate(backend, validBackendFixture())
	if !hasFieldError(errs, "spec.protocol", field.ErrorTypeRequired) {
		t.Fatalf("expected required error for spec.protocol, got %v", errs)
	}
}

func TestValidateBackendUpdateAllowsMissingProtocolForLegacyObject(t *testing.T) {
	backend := validBackendFixture()
	backend.Spec.Protocol = ""
	old := validBackendFixture()
	old.Spec.Protocol = ""

	if errs := ValidateBackendUpdate(backend, old); len(errs) != 0 {
		t.Fatalf("expected no validation errors for legacy backend, got %v", errs)
	}
}

func TestValidateBackendUpdateAllowsValidProtocol(t *testing.T) {
	backend := validBackendFixture()
	backend.Spec.Protocol = gatewayv1alpha1.BackendProtocolGRPC

	if errs := ValidateBackendUpdate(backend, validBackendFixture()); len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %v", errs)
	}
}

func validBackendFixture() *gatewayv1alpha1.Backend {
	return &gatewayv1alpha1.Backend{
		Spec: gatewayv1alpha1.BackendSpec{
			Type:        "Static",
			Protocol:    gatewayv1alpha1.BackendProtocolHTTP,
			DefaultPort: 8080,
			Static: &gatewayv1alpha1.StaticBackendSpec{
				Endpoints: []gatewayv1alpha1.BackendEndpoint{
					{Address: "127.0.0.1", Port: 8080, Weight: 100},
				},
			},
		},
	}
}

func hasFieldError(errs field.ErrorList, fieldPath string, errorType field.ErrorType) bool {
	for _, err := range errs {
		if err.Field == fieldPath && err.Type == errorType {
			return true
		}
	}
	return false
}
