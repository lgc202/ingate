package biz

import (
	"context"
	"fmt"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// GatewayLister 定义跨策略目标解析所需的 Gateway 列表能力
type GatewayLister interface {
	List(context.Context) ([]resource.Gateway, error)
}

// RouteLister 定义跨策略目标解析所需的 Route 列表能力
type RouteLister interface {
	List(context.Context) ([]resource.Route, error)
}

// PolicyTargetKey 唯一标识一个策略作用目标
type PolicyTargetKey struct {
	Kind resource.Kind
	ID   string
}

// PolicyTargetNames 保存策略作用目标的展示名称
type PolicyTargetNames map[PolicyTargetKey]string

// Name 返回目标引用对应的展示名称
func (n PolicyTargetNames) Name(ref resource.PolicyTargetRef) string {
	return n[PolicyTargetKey{Kind: ref.Kind, ID: ref.Name}]
}

// Contains 判断目标引用当前是否存在
func (n PolicyTargetNames) Contains(ref resource.PolicyTargetRef) bool {
	_, exists := n[PolicyTargetKey{Kind: ref.Kind, ID: ref.Name}]
	return exists
}

// PolicyTargetResolver 解析 Gateway 和 Route 策略作用目标
type PolicyTargetResolver struct {
	gateways GatewayLister
	routes   RouteLister
}

// NewPolicyTargetResolver 创建策略作用目标解析器
func NewPolicyTargetResolver(gateways GatewayLister, routes RouteLister) *PolicyTargetResolver {
	return &PolicyTargetResolver{gateways: gateways, routes: routes}
}

// Validate 校验所有策略作用目标是否存在
func (r *PolicyTargetResolver) Validate(ctx context.Context, refs []resource.PolicyTargetRef) error {
	names, err := r.DisplayNames(ctx, refs)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if names.Contains(ref) {
			continue
		}
		switch ref.Kind {
		case resource.KindGateway:
			return NewUserError(fmt.Sprintf("网关 %q 不存在", ref.Name))
		case resource.KindRoute:
			return NewUserError(fmt.Sprintf("路由 %q 不存在", ref.Name))
		default:
			return NewUserError("策略作用目标只支持网关或路由")
		}
	}
	return nil
}

// DisplayNames 返回当前存在的策略作用目标展示名称，缺失引用保留为空名称供状态页展示
func (r *PolicyTargetResolver) DisplayNames(ctx context.Context, refs []resource.PolicyTargetRef) (PolicyTargetNames, error) {
	names := make(PolicyTargetNames)
	needGateways := false
	needRoutes := false
	for _, ref := range refs {
		switch ref.Kind {
		case resource.KindGateway:
			needGateways = true
		case resource.KindRoute:
			needRoutes = true
		}
	}

	if needGateways {
		gateways, err := r.gateways.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, gateway := range gateways {
			names[PolicyTargetKey{Kind: resource.KindGateway, ID: gateway.Name}] = gateway.Spec.DisplayName
		}
	}
	if needRoutes {
		routes, err := r.routes.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, route := range routes {
			names[PolicyTargetKey{Kind: resource.KindRoute, ID: route.Name}] = route.Spec.DisplayName
		}
	}
	return names, nil
}
