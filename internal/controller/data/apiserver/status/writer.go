// Package status 将编译和发布结果写入声明式资源状态。
package status

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	gatewayclient "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned/typed/gateway/v1"
)

type resourceKey struct {
	kind gatewayv1.Kind
	name string
}

// Writer 将编译和发布结果收敛为声明式资源 Conditions。
type Writer struct {
	client gatewayclient.GatewayV1Interface
}

// NewWriter 创建声明式资源状态写入器。
func NewWriter(client gatewayclient.GatewayV1Interface) *Writer {
	return &Writer{client: client}
}

// ApplyCompileResult 更新本次资源集合的 Accepted、ResolvedRefs 和 Programmed Conditions。
func (w *Writer) ApplyCompileResult(
	ctx context.Context,
	resources compiler.Resources,
	diagnostics []compiler.Diagnostic,
	deliveryStatus delivery.Status,
) error {
	generations := resources.Generations()
	decisions := newDiagnosticIndex(generations, diagnostics)
	targets := newPolicyTargetIndex(generations)
	deliveryState := newDeliveryIndex(deliveryStatus)
	for _, resource := range generations {
		decision := decisions.forResource(resource.Kind, resource.Name)
		if resource.Kind == gatewayv1.KindWasmPlugin {
			decision = decisions.forWasmPlugin(resource.Name)
		}
		if err := w.updateResource(ctx, resource, &decision, deliveryState, targets); err != nil {
			return err
		}
	}
	return nil
}

// ApplyProgrammed 根据最新 Delivery 状态更新本次资源集合的 Programmed Condition。
func (w *Writer) ApplyProgrammed(
	ctx context.Context,
	resources compiler.Resources,
	deliveryStatus delivery.Status,
) error {
	generations := resources.Generations()
	targets := newPolicyTargetIndex(generations)
	deliveryState := newDeliveryIndex(deliveryStatus)
	for _, resource := range generations {
		if err := w.updateResource(ctx, resource, nil, deliveryState, targets); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) updateResource(
	ctx context.Context,
	resource compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryState deliveryIndex,
	targets map[resourceKey]compiler.ResourceGeneration,
) error {
	switch resource.Kind {
	case gatewayv1.KindGateway:
		return w.updateGateway(ctx, resource, compile, deliveryState)
	case gatewayv1.KindCertificate:
		return w.updateCertificate(ctx, resource, compile, deliveryState)
	case gatewayv1.KindRoute:
		return w.updateRoute(ctx, resource, compile, deliveryState)
	case gatewayv1.KindUpstream:
		return w.updateUpstream(ctx, resource, compile, deliveryState)
	case gatewayv1.KindRateLimitPolicy:
		return w.updateRateLimitPolicy(ctx, resource, compile, deliveryState, targets)
	case gatewayv1.KindIPRestrictionPolicy:
		return w.updateIPRestrictionPolicy(ctx, resource, compile, deliveryState, targets)
	case gatewayv1.KindHeaderTransformationPolicy:
		return w.updateHeaderTransformationPolicy(ctx, resource, compile, deliveryState, targets)
	case gatewayv1.KindMockResponsePolicy:
		return w.updateMockResponsePolicy(ctx, resource, compile, deliveryState, targets)
	case gatewayv1.KindWasmPlugin:
		return w.updateWasmPlugin(ctx, resource, compile)
	default:
		return fmt.Errorf("update unsupported resource kind %q", resource.Kind)
	}
}
