package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestValidateResolvedGatewayRejectsDuplicateListenerNames(t *testing.T) {
	resolvedGateway := validResolvedGatewayFixture()
	resolvedGateway.Spec.Listeners = []gatewayv1alpha1.ResolvedGatewayListener{
		{Name: "listener-a"},
		{Name: "listener-a"},
	}

	errs := ValidateResolvedGateway(resolvedGateway)
	if !hasFieldError(errs, "spec.listeners[1].name", field.ErrorTypeDuplicate) {
		t.Fatalf("expected duplicate error for spec.listeners[1].name, got %v", errs)
	}
}

func TestValidateResolvedGatewayRequiresListenerName(t *testing.T) {
	resolvedGateway := validResolvedGatewayFixture()
	resolvedGateway.Spec.Listeners = []gatewayv1alpha1.ResolvedGatewayListener{
		{},
	}

	errs := ValidateResolvedGateway(resolvedGateway)
	if !hasFieldError(errs, "spec.listeners[0].name", field.ErrorTypeRequired) {
		t.Fatalf("expected required error for spec.listeners[0].name, got %v", errs)
	}
}

func TestValidateResolvedGatewayRejectsDuplicateRouteNames(t *testing.T) {
	resolvedGateway := validResolvedGatewayFixture()
	resolvedGateway.Spec.Routes = []gatewayv1alpha1.ResolvedGatewayRoute{
		{Name: "route-a"},
		{Name: "route-a"},
	}

	errs := ValidateResolvedGateway(resolvedGateway)
	if !hasFieldError(errs, "spec.routes[1].name", field.ErrorTypeDuplicate) {
		t.Fatalf("expected duplicate error for spec.routes[1].name, got %v", errs)
	}
}

func TestValidateResolvedGatewayRequiresRouteName(t *testing.T) {
	resolvedGateway := validResolvedGatewayFixture()
	resolvedGateway.Spec.Routes = []gatewayv1alpha1.ResolvedGatewayRoute{
		{},
	}

	errs := ValidateResolvedGateway(resolvedGateway)
	if !hasFieldError(errs, "spec.routes[0].name", field.ErrorTypeRequired) {
		t.Fatalf("expected required error for spec.routes[0].name, got %v", errs)
	}
}

func TestValidateResolvedGatewayRejectsDuplicateBackendNames(t *testing.T) {
	resolvedGateway := validResolvedGatewayFixture()
	resolvedGateway.Spec.Backends = []gatewayv1alpha1.ResolvedGatewayBackend{
		{Name: "backend-a"},
		{Name: "backend-a"},
	}

	errs := ValidateResolvedGateway(resolvedGateway)
	if !hasFieldError(errs, "spec.backends[1].name", field.ErrorTypeDuplicate) {
		t.Fatalf("expected duplicate error for spec.backends[1].name, got %v", errs)
	}
}

func TestValidateResolvedGatewayRequiresBackendName(t *testing.T) {
	resolvedGateway := validResolvedGatewayFixture()
	resolvedGateway.Spec.Backends = []gatewayv1alpha1.ResolvedGatewayBackend{
		{},
	}

	errs := ValidateResolvedGateway(resolvedGateway)
	if !hasFieldError(errs, "spec.backends[0].name", field.ErrorTypeRequired) {
		t.Fatalf("expected required error for spec.backends[0].name, got %v", errs)
	}
}

func TestValidateResolvedGatewayAllowsValidObject(t *testing.T) {
	if errs := ValidateResolvedGateway(validResolvedGatewayFixture()); len(errs) != 0 {
		t.Fatalf("expected valid resolved gateway, got %v", errs)
	}
}

func TestValidateResolvedGatewayStatusUpdateRejectsObservedGenerationAboveGeneration(t *testing.T) {
	resolvedGateway := validResolvedGatewayFixture()
	resolvedGateway.Generation = 3
	resolvedGateway.Status.ObservedGeneration = 4
	resolvedGateway.Status.Conditions = []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Ready",
		},
	}

	old := validResolvedGatewayFixture()
	old.Generation = 3

	errs := ValidateResolvedGatewayStatusUpdate(resolvedGateway, old)
	if !hasFieldError(errs, "status.observedGeneration", field.ErrorTypeInvalid) {
		t.Fatalf("expected invalid error for status.observedGeneration, got %v", errs)
	}
}

func TestValidateResolvedGatewayStatusUpdateAllowsValidStatus(t *testing.T) {
	resolvedGateway := validResolvedGatewayFixture()
	resolvedGateway.Generation = 3
	resolvedGateway.Status.ObservedGeneration = 3
	resolvedGateway.Status.Conditions = []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Ready",
		},
	}

	old := validResolvedGatewayFixture()
	old.Generation = 3

	if errs := ValidateResolvedGatewayStatusUpdate(resolvedGateway, old); len(errs) != 0 {
		t.Fatalf("expected valid resolved gateway status update, got %v", errs)
	}
}

func TestValidateResolvedGatewayStatusUpdateRejectsNegativeObservedGeneration(t *testing.T) {
	resolvedGateway := validResolvedGatewayFixture()
	resolvedGateway.Generation = 3
	resolvedGateway.Status.ObservedGeneration = -1
	resolvedGateway.Status.Conditions = []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Ready",
		},
	}

	old := validResolvedGatewayFixture()
	old.Generation = 3

	errs := ValidateResolvedGatewayStatusUpdate(resolvedGateway, old)
	if !hasFieldError(errs, "status.observedGeneration", field.ErrorTypeInvalid) {
		t.Fatalf("expected invalid error for status.observedGeneration, got %v", errs)
	}
}

func TestValidateResolvedGatewayStatusUpdateRejectsInvalidConditions(t *testing.T) {
	resolvedGateway := validResolvedGatewayFixture()
	resolvedGateway.Generation = 3
	resolvedGateway.Status.ObservedGeneration = 3
	resolvedGateway.Status.Conditions = []metav1.Condition{{}}

	old := validResolvedGatewayFixture()
	old.Generation = 3

	errs := ValidateResolvedGatewayStatusUpdate(resolvedGateway, old)
	if !hasFieldError(errs, "status.conditions[0].status", field.ErrorTypeNotSupported) {
		t.Fatalf("expected not-supported error for status.conditions[0].status, got %v", errs)
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

func validResolvedGatewayFixture() *gatewayv1alpha1.ResolvedGateway {
	return &gatewayv1alpha1.ResolvedGateway{
		Spec: gatewayv1alpha1.ResolvedGatewaySpec{
			GatewayRef: gatewayv1alpha1.LocalObjectReference{Name: "gateway-a"},
			Version:    "v1",
			Listeners: []gatewayv1alpha1.ResolvedGatewayListener{
				{Name: "listener-a"},
			},
			Routes: []gatewayv1alpha1.ResolvedGatewayRoute{
				{Name: "route-a"},
			},
			Backends: []gatewayv1alpha1.ResolvedGatewayBackend{
				{Name: "backend-a"},
			},
			Extensions: []gatewayv1alpha1.ResolvedGatewayExtension{
				{Name: "ext-a"},
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
