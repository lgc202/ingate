package status

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiutil "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

const (
	ConditionAccepted = "Accepted"
	ConditionResolved = "Resolved"

	reasonResolved         = "Resolved"
	reasonResolveFailed    = "ResolveFailed"
	successMessageAccepted = "gateway was accepted for reconciliation"
	successMessageResolved = "gateway resources were resolved successfully"
)

type Updater struct {
	client clientset.Interface
	now    func() metav1.Time
}

func NewUpdater(client clientset.Interface) *Updater {
	return &Updater{client: client, now: func() metav1.Time { return metav1.NewTime(time.Now()) }}
}

func (u *Updater) MarkSuccess(
	ctx context.Context,
	gateway *gatewayv1alpha1.Gateway,
	routes []*gatewayv1alpha1.Route,
	backends []*gatewayv1alpha1.Backend,
	certificates []*gatewayv1alpha1.Certificate,
	authPolicies []*policyv1alpha1.AuthPolicy,
	trafficPolicies []*policyv1alpha1.TrafficPolicy,
) error {
	if u == nil || u.client == nil {
		return fmt.Errorf("status updater is not initialized")
	}
	if gateway == nil {
		return fmt.Errorf("success status update requires gateway")
	}

	if err := u.updateGatewayStatus(ctx, gateway, metav1.ConditionTrue, successMessageAccepted, metav1.ConditionTrue, successMessageResolved); err != nil {
		return err
	}
	for _, route := range routes {
		if route == nil {
			continue
		}
		if err := u.updateRouteStatus(ctx, route, metav1.ConditionTrue, successMessageAccepted, metav1.ConditionTrue, successMessageResolved); err != nil {
			return err
		}
	}
	for _, backend := range backends {
		if backend == nil {
			continue
		}
		if err := u.updateBackendStatus(ctx, backend, metav1.ConditionTrue, successMessageAccepted, metav1.ConditionTrue, successMessageResolved); err != nil {
			return err
		}
	}
	for _, certificate := range certificates {
		if certificate == nil {
			continue
		}
		if err := u.updateCertificateStatus(ctx, certificate, metav1.ConditionTrue, successMessageAccepted, metav1.ConditionTrue, successMessageResolved); err != nil {
			return err
		}
	}
	for _, policy := range authPolicies {
		if policy == nil {
			continue
		}
		if err := u.updateAuthPolicyStatus(ctx, policy, metav1.ConditionTrue, successMessageAccepted, metav1.ConditionTrue, successMessageResolved); err != nil {
			return err
		}
	}
	for _, policy := range trafficPolicies {
		if policy == nil {
			continue
		}
		if err := u.updateTrafficPolicyStatus(ctx, policy, metav1.ConditionTrue, successMessageAccepted, metav1.ConditionTrue, successMessageResolved); err != nil {
			return err
		}
	}
	return nil
}

func (u *Updater) MarkFailure(ctx context.Context, key shared.ObjectKey, err error) error {
	if u == nil || u.client == nil {
		return fmt.Errorf("status updater is not initialized")
	}
	message := err.Error()
	var statusErrs []error
	gateway, getErr := u.findGateway(ctx, key.Name)
	if getErr != nil {
		statusErrs = append(statusErrs, fmt.Errorf("find gateway %q: %w", key.Name, getErr))
	}
	if gateway != nil {
		if updateErr := u.updateGatewayStatus(ctx, gateway, metav1.ConditionFalse, message, metav1.ConditionFalse, message); updateErr != nil {
			statusErrs = append(statusErrs, fmt.Errorf("update gateway %q status: %w", gateway.Name, updateErr))
		}
		resources, resolveErr := u.resolveFailureResources(ctx, gateway)
		if resolveErr != nil {
			statusErrs = append(statusErrs, fmt.Errorf("resolve related resources for gateway %q: %w", gateway.Name, resolveErr))
		} else {
			for _, route := range resources.routes {
				if updateErr := u.updateRouteStatus(ctx, route, metav1.ConditionFalse, message, metav1.ConditionFalse, message); updateErr != nil {
					statusErrs = append(statusErrs, fmt.Errorf("update route %q status: %w", route.Name, updateErr))
				}
			}
			for _, backend := range resources.backends {
				if updateErr := u.updateBackendStatus(ctx, backend, metav1.ConditionFalse, message, metav1.ConditionFalse, message); updateErr != nil {
					statusErrs = append(statusErrs, fmt.Errorf("update backend %q status: %w", backend.Name, updateErr))
				}
			}
			for _, certificate := range resources.certificates {
				if updateErr := u.updateCertificateStatus(ctx, certificate, metav1.ConditionFalse, message, metav1.ConditionFalse, message); updateErr != nil {
					statusErrs = append(statusErrs, fmt.Errorf("update certificate %q status: %w", certificate.Name, updateErr))
				}
			}
			for _, policy := range resources.authPolicies {
				if updateErr := u.updateAuthPolicyStatus(ctx, policy, metav1.ConditionFalse, message, metav1.ConditionFalse, message); updateErr != nil {
					statusErrs = append(statusErrs, fmt.Errorf("update authpolicy %q status: %w", policy.Name, updateErr))
				}
			}
			for _, policy := range resources.trafficPolicies {
				if updateErr := u.updateTrafficPolicyStatus(ctx, policy, metav1.ConditionFalse, message, metav1.ConditionFalse, message); updateErr != nil {
					statusErrs = append(statusErrs, fmt.Errorf("update trafficpolicy %q status: %w", policy.Name, updateErr))
				}
			}
		}
	}

	return utilerrors.NewAggregate(statusErrs)
}

type failureResources struct {
	routes          []*gatewayv1alpha1.Route
	backends        []*gatewayv1alpha1.Backend
	certificates    []*gatewayv1alpha1.Certificate
	authPolicies    []*policyv1alpha1.AuthPolicy
	trafficPolicies []*policyv1alpha1.TrafficPolicy
}

func (u *Updater) findGateway(ctx context.Context, name string) (*gatewayv1alpha1.Gateway, error) {
	gateway, err := u.client.GatewayV1alpha1().Gateways().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return gateway, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}
	items, listErr := u.client.GatewayV1alpha1().Gateways().List(ctx, metav1.ListOptions{})
	if listErr != nil {
		return nil, listErr
	}
	for i := range items.Items {
		if items.Items[i].Name == name {
			return items.Items[i].DeepCopy(), nil
		}
	}
	return nil, nil
}

func (u *Updater) resolveFailureResources(ctx context.Context, gateway *gatewayv1alpha1.Gateway) (*failureResources, error) {
	if gateway == nil {
		return &failureResources{}, nil
	}
	resources := &failureResources{}
	routeNames := map[string]struct{}{}
	backendNames := map[string]struct{}{}
	certificateNames := map[string]struct{}{}

	routes, err := u.client.GatewayV1alpha1().Routes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range routes.Items {
		route := routes.Items[i].DeepCopy()
		if !routeTargetsGateway(route, gateway.Name) {
			continue
		}
		resources.routes = append(resources.routes, route)
		routeNames[route.Name] = struct{}{}
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.BackendRefs {
				if ref.Name != "" {
					backendNames[ref.Name] = struct{}{}
				}
			}
		}
	}

	backends, err := u.client.GatewayV1alpha1().Backends().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range backends.Items {
		backend := backends.Items[i].DeepCopy()
		if _, ok := backendNames[backend.Name]; ok {
			resources.backends = append(resources.backends, backend)
		}
	}

	for _, listener := range gateway.Spec.Listeners {
		if listener.TLS != nil && listener.TLS.CertificateRef != nil && listener.TLS.CertificateRef.Name != "" {
			certificateNames[listener.TLS.CertificateRef.Name] = struct{}{}
		}
	}
	certificates, err := u.client.GatewayV1alpha1().Certificates().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range certificates.Items {
		certificate := certificates.Items[i].DeepCopy()
		if _, ok := certificateNames[certificate.Name]; ok {
			resources.certificates = append(resources.certificates, certificate)
		}
	}

	authPolicies, err := u.client.PolicyV1alpha1().AuthPolicies().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range authPolicies.Items {
		policy := authPolicies.Items[i].DeepCopy()
		if policyTargetsResources(policy.Spec.TargetRefs, gateway.Name, routeNames, backendNames) {
			resources.authPolicies = append(resources.authPolicies, policy)
		}
	}

	trafficPolicies, err := u.client.PolicyV1alpha1().TrafficPolicies().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range trafficPolicies.Items {
		policy := trafficPolicies.Items[i].DeepCopy()
		if policyTargetsResources(policy.Spec.TargetRefs, gateway.Name, routeNames, backendNames) {
			resources.trafficPolicies = append(resources.trafficPolicies, policy)
		}
	}

	return resources, nil
}

func routeTargetsGateway(route *gatewayv1alpha1.Route, gatewayName string) bool {
	for _, parent := range route.Spec.ParentRefs {
		if parent.Name == gatewayName {
			return true
		}
	}
	return false
}

func policyTargetsResources(targets []policyv1alpha1.TargetReference, gatewayName string, routeNames map[string]struct{}, backendNames map[string]struct{}) bool {
	for _, target := range targets {
		switch target.Kind {
		case "Gateway":
			if target.Name == gatewayName {
				return true
			}
		case "Route":
			if _, ok := routeNames[target.Name]; ok {
				return true
			}
		case "Backend":
			if _, ok := backendNames[target.Name]; ok {
				return true
			}
		}
	}
	return false
}

func (u *Updater) updateGatewayStatus(ctx context.Context, gateway *gatewayv1alpha1.Gateway, acceptedStatus metav1.ConditionStatus, acceptedMessage string, resolvedStatus metav1.ConditionStatus, resolvedMessage string) error {
	updated := gateway.DeepCopy()
	updated.Status.ObservedGeneration = updated.Generation
	setCondition(&updated.Status.Conditions, ConditionAccepted, acceptedStatus, conditionReason(acceptedStatus), acceptedMessage, u.now())
	setCondition(&updated.Status.Conditions, ConditionResolved, resolvedStatus, conditionReason(resolvedStatus), resolvedMessage, u.now())
	_, err := u.client.GatewayV1alpha1().Gateways().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsNotFound(err) {
		_, err = u.client.GatewayV1alpha1().Gateways().Update(ctx, updated, metav1.UpdateOptions{})
	}
	return err
}

func (u *Updater) updateRouteStatus(ctx context.Context, route *gatewayv1alpha1.Route, acceptedStatus metav1.ConditionStatus, acceptedMessage string, resolvedStatus metav1.ConditionStatus, resolvedMessage string) error {
	updated := route.DeepCopy()
	updated.Status.ObservedGeneration = updated.Generation
	setCondition(&updated.Status.Conditions, ConditionAccepted, acceptedStatus, conditionReason(acceptedStatus), acceptedMessage, u.now())
	setCondition(&updated.Status.Conditions, ConditionResolved, resolvedStatus, conditionReason(resolvedStatus), resolvedMessage, u.now())
	_, err := u.client.GatewayV1alpha1().Routes().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsNotFound(err) {
		_, err = u.client.GatewayV1alpha1().Routes().Update(ctx, updated, metav1.UpdateOptions{})
	}
	return err
}

func (u *Updater) updateBackendStatus(ctx context.Context, backend *gatewayv1alpha1.Backend, acceptedStatus metav1.ConditionStatus, acceptedMessage string, resolvedStatus metav1.ConditionStatus, resolvedMessage string) error {
	updated := backend.DeepCopy()
	updated.Status.ObservedGeneration = updated.Generation
	setCondition(&updated.Status.Conditions, ConditionAccepted, acceptedStatus, conditionReason(acceptedStatus), acceptedMessage, u.now())
	setCondition(&updated.Status.Conditions, ConditionResolved, resolvedStatus, conditionReason(resolvedStatus), resolvedMessage, u.now())
	_, err := u.client.GatewayV1alpha1().Backends().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsNotFound(err) {
		_, err = u.client.GatewayV1alpha1().Backends().Update(ctx, updated, metav1.UpdateOptions{})
	}
	return err
}

func (u *Updater) updateCertificateStatus(ctx context.Context, certificate *gatewayv1alpha1.Certificate, acceptedStatus metav1.ConditionStatus, acceptedMessage string, resolvedStatus metav1.ConditionStatus, resolvedMessage string) error {
	updated := certificate.DeepCopy()
	updated.Status.ObservedGeneration = updated.Generation
	setCondition(&updated.Status.Conditions, ConditionAccepted, acceptedStatus, conditionReason(acceptedStatus), acceptedMessage, u.now())
	setCondition(&updated.Status.Conditions, ConditionResolved, resolvedStatus, conditionReason(resolvedStatus), resolvedMessage, u.now())
	_, err := u.client.GatewayV1alpha1().Certificates().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsNotFound(err) {
		_, err = u.client.GatewayV1alpha1().Certificates().Update(ctx, updated, metav1.UpdateOptions{})
	}
	return err
}

func (u *Updater) updateAuthPolicyStatus(ctx context.Context, policy *policyv1alpha1.AuthPolicy, acceptedStatus metav1.ConditionStatus, acceptedMessage string, resolvedStatus metav1.ConditionStatus, resolvedMessage string) error {
	updated := policy.DeepCopy()
	updated.Status.ObservedGeneration = updated.Generation
	setCondition(&updated.Status.Conditions, ConditionAccepted, acceptedStatus, conditionReason(acceptedStatus), acceptedMessage, u.now())
	setCondition(&updated.Status.Conditions, ConditionResolved, resolvedStatus, conditionReason(resolvedStatus), resolvedMessage, u.now())
	_, err := u.client.PolicyV1alpha1().AuthPolicies().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsNotFound(err) {
		_, err = u.client.PolicyV1alpha1().AuthPolicies().Update(ctx, updated, metav1.UpdateOptions{})
	}
	return err
}

func (u *Updater) updateTrafficPolicyStatus(ctx context.Context, policy *policyv1alpha1.TrafficPolicy, acceptedStatus metav1.ConditionStatus, acceptedMessage string, resolvedStatus metav1.ConditionStatus, resolvedMessage string) error {
	updated := policy.DeepCopy()
	updated.Status.ObservedGeneration = updated.Generation
	setCondition(&updated.Status.Conditions, ConditionAccepted, acceptedStatus, conditionReason(acceptedStatus), acceptedMessage, u.now())
	setCondition(&updated.Status.Conditions, ConditionResolved, resolvedStatus, conditionReason(resolvedStatus), resolvedMessage, u.now())
	_, err := u.client.PolicyV1alpha1().TrafficPolicies().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsNotFound(err) {
		_, err = u.client.PolicyV1alpha1().TrafficPolicies().Update(ctx, updated, metav1.UpdateOptions{})
	}
	return err
}

func setCondition(conditions *[]metav1.Condition, conditionType string, status metav1.ConditionStatus, reason, message string, now metav1.Time) {
	apiutil.SetStatusCondition(conditions, metav1.Condition{Type: conditionType, Status: status, Reason: reason, Message: message, LastTransitionTime: now})
}

func conditionReason(status metav1.ConditionStatus) string {
	if status == metav1.ConditionTrue {
		return reasonResolved
	}
	return reasonResolveFailed
}
