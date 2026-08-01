// Package status 将编译和发布结果写入声明式资源状态
package status

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/controller/compiler"
	"github.com/lgc202/ingate/internal/controller/delivery"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1"
)

type resourceKey struct {
	kind gatewayv1.Kind
	name string
}

// Writer 将编译和发布结果收敛为声明式资源 Conditions
type Writer struct {
	client gatewayclient.GatewayV1Interface
}

// NewWriter 创建声明式资源状态写入器
func NewWriter(client gatewayclient.GatewayV1Interface) *Writer {
	return &Writer{client: client}
}

// ApplyCompileResult 更新本次资源集合的 Accepted、ResolvedRefs 和 Programmed Conditions
func (w *Writer) ApplyCompileResult(
	ctx context.Context,
	resources compiler.Resources,
	diagnostics []compiler.Diagnostic,
	deliveryStatus delivery.Status,
) error {
	decisions := newDiagnosticIndex(resources, diagnostics)
	targets := newPolicyTargetIndex(resources)
	programmedTargets := newProgrammedPolicyTargetIndex(deliveryStatus.ActivePolicyTargets)
	for _, resource := range resources.Generations() {
		decision := decisions.forResource(resource.Kind, resource.Name)
		if err := w.updateResource(ctx, resource, &decision, deliveryStatus, targets, programmedTargets); err != nil {
			return err
		}
	}
	return nil
}

// ApplyProgrammed 根据最新 Delivery 状态更新本次资源集合的 Programmed Condition
func (w *Writer) ApplyProgrammed(
	ctx context.Context,
	resources compiler.Resources,
	deliveryStatus delivery.Status,
) error {
	targets := newPolicyTargetIndex(resources)
	programmedTargets := newProgrammedPolicyTargetIndex(deliveryStatus.ActivePolicyTargets)
	for _, resource := range resources.Generations() {
		if err := w.updateResource(ctx, resource, nil, deliveryStatus, targets, programmedTargets); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) updateResource(
	ctx context.Context,
	resource compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
	targets map[resourceKey]compiler.ResourceGeneration,
	programmedTargets map[compiler.CompiledPolicyTarget]bool,
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
		return w.updateRateLimitPolicy(ctx, resource, compile, deliveryStatus, targets, programmedTargets)
	case gatewayv1.KindAccessControlPolicy:
		return w.updateAccessControlPolicy(ctx, resource, compile, deliveryStatus, targets, programmedTargets)
	case gatewayv1.KindTokenQuotaPolicy:
		return w.updateTokenQuotaPolicy(ctx, resource, compile, deliveryStatus, targets, programmedTargets)
	default:
		return fmt.Errorf("update unsupported resource kind %q", resource.Kind)
	}
}
