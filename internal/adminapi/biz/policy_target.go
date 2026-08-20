package biz

import (
	"context"
	"errors"
	"fmt"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// GatewayGetter 定义策略目标解析所需的 Gateway 查询能力
type GatewayGetter interface {
	Get(context.Context, string) (*resource.Gateway, error)
}

// RouteGetter 定义策略目标解析所需的 Route 查询能力
type RouteGetter interface {
	Get(context.Context, string) (*resource.Route, error)
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
	gateways GatewayGetter
	routes   RouteGetter
}

// NewPolicyTargetResolver 创建策略作用目标解析器
func NewPolicyTargetResolver(gateways GatewayGetter, routes RouteGetter) *PolicyTargetResolver {
	return &PolicyTargetResolver{gateways: gateways, routes: routes}
}

// Validate 校验所有策略作用目标是否存在
func (r *PolicyTargetResolver) Validate(ctx context.Context, refs []resource.PolicyTargetRef) error {
	_, err := r.Resolve(ctx, refs)
	return err
}

// Resolve 校验策略作用目标并返回当前展示名称
func (r *PolicyTargetResolver) Resolve(ctx context.Context, refs []resource.PolicyTargetRef) (PolicyTargetNames, error) {
	names, err := r.DisplayNames(ctx, refs)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if names.Contains(ref) {
			continue
		}
		switch ref.Kind {
		case resource.KindGateway:
			return nil, NewUserError(fmt.Sprintf("网关 %q 不存在", ref.Name))
		case resource.KindRoute:
			return nil, NewUserError(fmt.Sprintf("路由 %q 不存在", ref.Name))
		default:
			return nil, NewUserError("策略作用目标只支持网关或路由")
		}
	}
	return names, nil
}

// DisplayNames 返回当前存在的策略作用目标展示名称，缺失引用保留为空名称供状态页展示
func (r *PolicyTargetResolver) DisplayNames(ctx context.Context, refs []resource.PolicyTargetRef) (PolicyTargetNames, error) {
	names := make(PolicyTargetNames)
	seen := make(map[PolicyTargetKey]struct{}, len(refs))
	for _, ref := range refs {
		key := PolicyTargetKey{Kind: ref.Kind, ID: ref.Name}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		switch ref.Kind {
		case resource.KindGateway:
			gateway, err := r.gateways.Get(ctx, ref.Name)
			if err != nil {
				if errors.Is(err, ErrResourceNotFound) {
					continue
				}
				return nil, err
			}
			names[key] = gateway.Spec.DisplayName
		case resource.KindRoute:
			route, err := r.routes.Get(ctx, ref.Name)
			if err != nil {
				if errors.Is(err, ErrResourceNotFound) {
					continue
				}
				return nil, err
			}
			names[key] = route.Spec.DisplayName
		}
	}
	return names, nil
}
