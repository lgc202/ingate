package status

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/envoy/config"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1"
)

const (
	conditionAccepted    = "Accepted"
	messageAccepted      = "Resource is accepted"
	messageCompileFailed = "Envoy configuration could not be compiled"
)

type resourceKey struct {
	kind gatewayv1.Kind
	id   string
}

type conditionDecision struct {
	accepted bool
	reason   config.Reason
	message  string
}

type conditionDecisions struct {
	specific map[resourceKey]conditionDecision
	kinds    map[gatewayv1.Kind]conditionDecision
	global   *conditionDecision
}

// AcceptedWriter 将编译诊断收敛为每个声明式资源唯一的 Accepted Condition
type AcceptedWriter struct {
	client gatewayclient.GatewayV1Interface
}

// NewAcceptedWriter 创建资源 Accepted 状态写入器
func NewAcceptedWriter(client clientset.Interface) *AcceptedWriter {
	return &AcceptedWriter{client: client.GatewayV1()}
}

// ApplyDiagnostics 更新本次 ResourceSet 中所有资源的 Accepted Condition
func (w *AcceptedWriter) ApplyDiagnostics(
	ctx context.Context,
	resources config.ResourceSet,
	diagnostics []config.Diagnostic,
) error {
	decisions := newConditionDecisions(resources, diagnostics)

	for _, resource := range resources.Gateways {
		if resource == nil {
			continue
		}
		if err := w.updateGateway(ctx, resource.Name, resource.Generation, decisions.forResource(gatewayv1.KindGateway, resource.Name)); err != nil {
			return err
		}
	}
	for _, resource := range resources.Routes {
		if resource == nil {
			continue
		}
		if err := w.updateRoute(ctx, resource.Name, resource.Generation, decisions.forResource(gatewayv1.KindRoute, resource.Name)); err != nil {
			return err
		}
	}
	for _, resource := range resources.Upstreams {
		if resource == nil {
			continue
		}
		if err := w.updateUpstream(ctx, resource.Name, resource.Generation, decisions.forResource(gatewayv1.KindUpstream, resource.Name)); err != nil {
			return err
		}
	}
	for _, resource := range resources.RateLimitPolicies {
		if resource == nil {
			continue
		}
		if err := w.updateRateLimitPolicy(ctx, resource.Name, resource.Generation, decisions.forResource(gatewayv1.KindRateLimitPolicy, resource.Name)); err != nil {
			return err
		}
	}
	for _, resource := range resources.AccessControlPolicies {
		if resource == nil {
			continue
		}
		if err := w.updateAccessControlPolicy(ctx, resource.Name, resource.Generation, decisions.forResource(gatewayv1.KindAccessControlPolicy, resource.Name)); err != nil {
			return err
		}
	}
	for _, resource := range resources.PolicyBindings {
		if resource == nil {
			continue
		}
		if err := w.updatePolicyBinding(ctx, resource.Name, resource.Generation, decisions.forResource(gatewayv1.KindPolicyBinding, resource.Name)); err != nil {
			return err
		}
	}
	return nil
}

func (w *AcceptedWriter) updateGateway(ctx context.Context, id string, generation int64, decision conditionDecision) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Gateways().Get(ctx, id, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.Generation != generation {
			return nil
		}

		conditions := acceptedConditions(resource.Status.Conditions, generation, decision)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.Gateways().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update Gateway %q Accepted condition: %w", id, err)
	}
	return nil
}

func (w *AcceptedWriter) updateRoute(ctx context.Context, id string, generation int64, decision conditionDecision) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Routes().Get(ctx, id, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.Generation != generation {
			return nil
		}

		conditions := acceptedConditions(resource.Status.Conditions, generation, decision)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.Routes().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update Route %q Accepted condition: %w", id, err)
	}
	return nil
}

func (w *AcceptedWriter) updateUpstream(ctx context.Context, id string, generation int64, decision conditionDecision) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Upstreams().Get(ctx, id, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.Generation != generation {
			return nil
		}

		conditions := acceptedConditions(resource.Status.Conditions, generation, decision)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.Upstreams().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update Upstream %q Accepted condition: %w", id, err)
	}
	return nil
}

func (w *AcceptedWriter) updateRateLimitPolicy(ctx context.Context, id string, generation int64, decision conditionDecision) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.RateLimitPolicies().Get(ctx, id, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.Generation != generation {
			return nil
		}

		conditions := acceptedConditions(resource.Status.Conditions, generation, decision)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.RateLimitPolicies().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update RateLimitPolicy %q Accepted condition: %w", id, err)
	}
	return nil
}

func (w *AcceptedWriter) updateAccessControlPolicy(ctx context.Context, id string, generation int64, decision conditionDecision) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.AccessControlPolicies().Get(ctx, id, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.Generation != generation {
			return nil
		}

		conditions := acceptedConditions(resource.Status.Conditions, generation, decision)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.AccessControlPolicies().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update AccessControlPolicy %q Accepted condition: %w", id, err)
	}
	return nil
}

func (w *AcceptedWriter) updatePolicyBinding(ctx context.Context, id string, generation int64, decision conditionDecision) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.PolicyBindings().Get(ctx, id, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.Generation != generation {
			return nil
		}

		conditions := acceptedConditions(resource.Status.Conditions, generation, decision)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.PolicyBindings().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update PolicyBinding %q Accepted condition: %w", id, err)
	}
	return nil
}

func newConditionDecisions(resources config.ResourceSet, diagnostics []config.Diagnostic) conditionDecisions {
	knownResources := make(map[resourceKey]bool)
	knownKinds := make(map[gatewayv1.Kind]bool)
	addKnownResources(knownResources, knownKinds, gatewayv1.KindGateway, resources.Gateways, func(resource *gatewayv1.Gateway) string { return resource.Name })
	addKnownResources(knownResources, knownKinds, gatewayv1.KindRoute, resources.Routes, func(resource *gatewayv1.Route) string { return resource.Name })
	addKnownResources(knownResources, knownKinds, gatewayv1.KindUpstream, resources.Upstreams, func(resource *gatewayv1.Upstream) string { return resource.Name })
	addKnownResources(knownResources, knownKinds, gatewayv1.KindRateLimitPolicy, resources.RateLimitPolicies, func(resource *gatewayv1.RateLimitPolicy) string { return resource.Name })
	addKnownResources(knownResources, knownKinds, gatewayv1.KindAccessControlPolicy, resources.AccessControlPolicies, func(resource *gatewayv1.AccessControlPolicy) string { return resource.Name })
	addKnownResources(knownResources, knownKinds, gatewayv1.KindPolicyBinding, resources.PolicyBindings, func(resource *gatewayv1.PolicyBinding) string { return resource.Name })

	decisions := conditionDecisions{
		specific: make(map[resourceKey]conditionDecision),
		kinds:    make(map[gatewayv1.Kind]conditionDecision),
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != config.SeverityError {
			continue
		}
		decision := conditionDecision{
			reason:  diagnostic.Reason,
			message: diagnostic.Message,
		}
		if decision.reason == "" {
			decision.reason = config.ReasonCompileFailed
		}
		if decision.message == "" {
			decision.message = messageCompileFailed
		}

		key := resourceKey{kind: diagnostic.Kind, id: diagnostic.ID}
		switch {
		case diagnostic.Kind == "" || !knownKinds[diagnostic.Kind]:
			if decisions.global == nil {
				decision.message = messageCompileFailed
				decisions.global = &decision
			}
		case diagnostic.ID == "" || !knownResources[key]:
			if _, exists := decisions.kinds[diagnostic.Kind]; !exists {
				decisions.kinds[diagnostic.Kind] = decision
			}
		default:
			if _, exists := decisions.specific[key]; !exists {
				decisions.specific[key] = decision
			}
		}
	}
	return decisions
}

func addKnownResources[T any](
	resources map[resourceKey]bool,
	kinds map[gatewayv1.Kind]bool,
	kind gatewayv1.Kind,
	items []*T,
	id func(*T) string,
) {
	for _, item := range items {
		if item == nil {
			continue
		}
		resources[resourceKey{kind: kind, id: id(item)}] = true
		kinds[kind] = true
	}
}

func (d conditionDecisions) forResource(kind gatewayv1.Kind, id string) conditionDecision {
	if decision, ok := d.specific[resourceKey{kind: kind, id: id}]; ok {
		return decision
	}
	if decision, ok := d.kinds[kind]; ok {
		return decision
	}
	if d.global != nil {
		return *d.global
	}
	return conditionDecision{
		accepted: true,
		reason:   config.ReasonAccepted,
		message:  messageAccepted,
	}
}

func acceptedConditions(existing []metav1.Condition, generation int64, decision conditionDecision) []metav1.Condition {
	status := metav1.ConditionFalse
	if decision.accepted {
		status = metav1.ConditionTrue
	}

	lastTransitionTime := metav1.Now()
	for i := range existing {
		if existing[i].Type != conditionAccepted {
			continue
		}
		if existing[i].Status == status && !existing[i].LastTransitionTime.IsZero() {
			lastTransitionTime = existing[i].LastTransitionTime
		}
		break
	}

	return []metav1.Condition{{
		Type:               conditionAccepted,
		Status:             status,
		ObservedGeneration: generation,
		LastTransitionTime: lastTransitionTime,
		Reason:             string(decision.reason),
		Message:            decision.message,
	}}
}
