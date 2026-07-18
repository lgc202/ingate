package status

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/envoy/config"
	"github.com/lgc202/ingate/internal/envoy/delivery"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1"
)

const (
	messageAccepted       = "Resource is accepted"
	messageResolvedRefs   = "All resource references are resolved"
	messageProgrammed     = "Resource configuration is active"
	messagePending        = "Resource configuration is pending"
	messageRejected       = "Resource configuration was rejected"
	messageDeliveryFailed = "Resource configuration could not be applied"
	messageCompileFailed  = "Resource configuration could not be compiled"
)

type resourceKey struct {
	kind gatewayv1.Kind
	name string
}

type conditionDecision struct {
	status  metav1.ConditionStatus
	reason  gatewayv1.ConditionReason
	message string
}

type compileDecision struct {
	accepted     conditionDecision
	resolvedRefs *conditionDecision
}

type diagnosticIndex struct {
	specific map[resourceKey][]config.Diagnostic
	kinds    map[gatewayv1.Kind][]config.Diagnostic
	global   []config.Diagnostic
}

// Writer 将编译和发布结果收敛为声明式资源 Conditions
type Writer struct {
	client gatewayclient.GatewayV1Interface
}

// NewWriter 创建声明式资源状态写入器
func NewWriter(client clientset.Interface) *Writer {
	return &Writer{client: client.GatewayV1()}
}

// ApplyCompileResult 更新本次资源集合的 Accepted、ResolvedRefs 和 Programmed Conditions
func (w *Writer) ApplyCompileResult(
	ctx context.Context,
	resources config.ResourceSet,
	diagnostics []config.Diagnostic,
	deliveryStatus delivery.Status,
) error {
	decisions := newDiagnosticIndex(resources, diagnostics)
	for _, resource := range resources.Generations() {
		decision := decisions.forResource(resource.Kind, resource.Name)
		if err := w.updateResource(ctx, resource, &decision, deliveryStatus); err != nil {
			return err
		}
	}
	return nil
}

// ApplyProgrammed 根据最新 Delivery 状态更新本次资源集合的 Programmed Condition
func (w *Writer) ApplyProgrammed(
	ctx context.Context,
	resources config.ResourceSet,
	deliveryStatus delivery.Status,
) error {
	for _, resource := range resources.Generations() {
		if err := w.updateResource(ctx, resource, nil, deliveryStatus); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) updateResource(
	ctx context.Context,
	resource config.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
) error {
	switch resource.Kind {
	case gatewayv1.KindGateway:
		return w.updateGateway(ctx, resource, compile, deliveryStatus)
	case gatewayv1.KindCertificate:
		return w.updateCertificate(ctx, resource, compile, deliveryStatus)
	case gatewayv1.KindRoute:
		return w.updateRoute(ctx, resource, compile, deliveryStatus)
	case gatewayv1.KindUpstream:
		return w.updateUpstream(ctx, resource, compile, deliveryStatus)
	case gatewayv1.KindRateLimitPolicy:
		return w.updateRateLimitPolicy(ctx, resource, compile, deliveryStatus)
	case gatewayv1.KindAccessControlPolicy:
		return w.updateAccessControlPolicy(ctx, resource, compile, deliveryStatus)
	case gatewayv1.KindPolicyBinding:
		return w.updatePolicyBinding(ctx, resource, compile, deliveryStatus)
	default:
		return fmt.Errorf("update unsupported resource kind %q", resource.Kind)
	}
}

func (w *Writer) updateGateway(
	ctx context.Context,
	source config.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Gateways().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryStatus)
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
		return fmt.Errorf("update Gateway %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateCertificate(
	ctx context.Context,
	source config.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Certificates().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryStatus)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.Certificates().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update Certificate %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateRoute(
	ctx context.Context,
	source config.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Routes().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryStatus)
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
		return fmt.Errorf("update Route %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateUpstream(
	ctx context.Context,
	source config.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Upstreams().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryStatus)
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
		return fmt.Errorf("update Upstream %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateRateLimitPolicy(
	ctx context.Context,
	source config.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.RateLimitPolicies().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryStatus)
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
		return fmt.Errorf("update RateLimitPolicy %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateAccessControlPolicy(
	ctx context.Context,
	source config.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.AccessControlPolicies().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryStatus)
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
		return fmt.Errorf("update AccessControlPolicy %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updatePolicyBinding(
	ctx context.Context,
	source config.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.PolicyBindings().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryStatus)
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
		return fmt.Errorf("update PolicyBinding %q conditions: %w", source.Name, err)
	}
	return nil
}

func resourceConditions(
	existing []metav1.Condition,
	resource config.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
) []metav1.Condition {
	conditions := slices.Clone(existing)
	if compile != nil {
		meta.SetStatusCondition(&conditions, newCondition(
			gatewayv1.ConditionAccepted,
			resource.Generation,
			compile.accepted,
		))
		if compile.resolvedRefs != nil {
			meta.SetStatusCondition(&conditions, newCondition(
				gatewayv1.ConditionResolvedRefs,
				resource.Generation,
				*compile.resolvedRefs,
			))
		}
	}

	meta.SetStatusCondition(&conditions, programmedCondition(conditions, resource, deliveryStatus))
	return conditions
}

func programmedCondition(
	conditions []metav1.Condition,
	resource config.ResourceGeneration,
	deliveryStatus delivery.Status,
) metav1.Condition {
	accepted := currentCondition(conditions, gatewayv1.ConditionAccepted, resource.Generation)
	if accepted == nil {
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, pendingDecision())
	}
	if accepted.Status != metav1.ConditionTrue {
		return conditionBlockedBy(gatewayv1.ConditionProgrammed, resource.Generation, accepted)
	}

	if kindHasReferences(resource.Kind) {
		resolvedRefs := currentCondition(conditions, gatewayv1.ConditionResolvedRefs, resource.Generation)
		if resolvedRefs == nil {
			return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, pendingDecision())
		}
		if resolvedRefs.Status != metav1.ConditionTrue {
			return conditionBlockedBy(gatewayv1.ConditionProgrammed, resource.Generation, resolvedRefs)
		}
	}

	if slices.Contains(deliveryStatus.ActiveResources, resource) {
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, conditionDecision{
			status:  metav1.ConditionTrue,
			reason:  gatewayv1.ReasonProgrammed,
			message: messageProgrammed,
		})
	}
	if deliveryStatus.LastFailure != nil && slices.Contains(deliveryStatus.LastFailure.Resources, resource) {
		reason := gatewayv1.ReasonDeliveryFailed
		message := messageDeliveryFailed
		if deliveryStatus.LastFailure.Reason == delivery.FailureRejected {
			reason = gatewayv1.ReasonRejected
			message = messageRejected
		}
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, conditionDecision{
			status:  metav1.ConditionFalse,
			reason:  reason,
			message: message,
		})
	}
	return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, pendingDecision())
}

func conditionBlockedBy(
	conditionType gatewayv1.ConditionType,
	generation int64,
	blocking *metav1.Condition,
) metav1.Condition {
	status := metav1.ConditionUnknown
	reason := gatewayv1.ReasonPending
	message := messagePending
	if blocking.Status == metav1.ConditionFalse {
		status = metav1.ConditionFalse
		reason = gatewayv1.ConditionReason(blocking.Reason)
		message = blocking.Message
	}
	return newCondition(conditionType, generation, conditionDecision{
		status:  status,
		reason:  reason,
		message: message,
	})
}

func currentCondition(
	conditions []metav1.Condition,
	conditionType gatewayv1.ConditionType,
	generation int64,
) *metav1.Condition {
	condition := meta.FindStatusCondition(conditions, string(conditionType))
	if condition == nil || condition.ObservedGeneration != generation {
		return nil
	}
	return condition
}

func newCondition(
	conditionType gatewayv1.ConditionType,
	generation int64,
	decision conditionDecision,
) metav1.Condition {
	return metav1.Condition{
		Type:               string(conditionType),
		Status:             decision.status,
		ObservedGeneration: generation,
		Reason:             string(decision.reason),
		Message:            decision.message,
	}
}

func pendingDecision() conditionDecision {
	return conditionDecision{
		status:  metav1.ConditionUnknown,
		reason:  gatewayv1.ReasonPending,
		message: messagePending,
	}
}

func newDiagnosticIndex(resources config.ResourceSet, diagnostics []config.Diagnostic) diagnosticIndex {
	knownResources := make(map[resourceKey]bool)
	knownKinds := make(map[gatewayv1.Kind]bool)
	for _, resource := range resources.Generations() {
		knownResources[resourceKey{kind: resource.Kind, name: resource.Name}] = true
		knownKinds[resource.Kind] = true
	}

	index := diagnosticIndex{
		specific: make(map[resourceKey][]config.Diagnostic),
		kinds:    make(map[gatewayv1.Kind][]config.Diagnostic),
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != config.SeverityError {
			continue
		}
		key := resourceKey{kind: diagnostic.Kind, name: diagnostic.ID}
		switch {
		case diagnostic.Kind == "" || !knownKinds[diagnostic.Kind]:
			index.global = append(index.global, diagnostic)
		case diagnostic.ID == "" || !knownResources[key]:
			index.kinds[diagnostic.Kind] = append(index.kinds[diagnostic.Kind], diagnostic)
		default:
			index.specific[key] = append(index.specific[key], diagnostic)
		}
	}
	return index
}

func (i diagnosticIndex) forResource(kind gatewayv1.Kind, name string) compileDecision {
	diagnostics := make([]config.Diagnostic, 0,
		len(i.global)+len(i.kinds[kind])+len(i.specific[resourceKey{kind: kind, name: name}]),
	)
	diagnostics = append(diagnostics, i.global...)
	diagnostics = append(diagnostics, i.kinds[kind]...)
	diagnostics = append(diagnostics, i.specific[resourceKey{kind: kind, name: name}]...)

	decision := compileDecision{
		accepted: conditionDecision{
			status:  metav1.ConditionTrue,
			reason:  gatewayv1.ReasonAccepted,
			message: messageAccepted,
		},
	}
	if kindHasReferences(kind) {
		resolvedRefs := conditionDecision{
			status:  metav1.ConditionTrue,
			reason:  gatewayv1.ReasonResolvedRefs,
			message: messageResolvedRefs,
		}
		decision.resolvedRefs = &resolvedRefs
	}

	for _, diagnostic := range diagnostics {
		if decision.resolvedRefs != nil && isReferenceReason(diagnostic.Reason) {
			if decision.resolvedRefs.status == metav1.ConditionTrue {
				*decision.resolvedRefs = decisionFromDiagnostic(diagnostic)
			}
			continue
		}
		if decision.accepted.status == metav1.ConditionTrue {
			decision.accepted = decisionFromDiagnostic(diagnostic)
		}
	}
	return decision
}

func decisionFromDiagnostic(diagnostic config.Diagnostic) conditionDecision {
	reason := diagnostic.Reason
	if reason == "" {
		reason = gatewayv1.ReasonCompileFailed
	}
	message := diagnostic.Message
	if message == "" || diagnostic.Kind == "" {
		message = messageCompileFailed
	}
	return conditionDecision{
		status:  metav1.ConditionFalse,
		reason:  reason,
		message: message,
	}
}

func isReferenceReason(reason config.Reason) bool {
	return reason == config.ReasonReferenceNotFound || reason == config.ReasonInvalidReference
}

func kindHasReferences(kind gatewayv1.Kind) bool {
	switch kind {
	case gatewayv1.KindGateway, gatewayv1.KindRoute, gatewayv1.KindPolicyBinding:
		return true
	default:
		return false
	}
}
