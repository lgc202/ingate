package biz

import (
	"context"
	"fmt"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// GatewayLister 定义策略目标批量解析所需的 Gateway 查询能力
type GatewayLister interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[resource.Gateway], error)
}

// RouteLister 定义策略目标批量解析所需的 Route 查询能力
type RouteLister interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[resource.Route], error)
}

// CallerLister 定义调用方策略目标批量解析所需的查询能力
type CallerLister interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[resource.Caller], error)
}

type policyTargetKey struct {
	Kind resource.Kind
	ID   string
}

// PolicyTargetNames 保存策略作用目标的展示名称
type PolicyTargetNames struct {
	values map[policyTargetKey]string
}

// Name 返回目标引用对应的展示名称
func (n PolicyTargetNames) Name(ref resource.PolicyTargetRef) string {
	return n.values[policyTargetKey{Kind: ref.Kind, ID: ref.Name}]
}

func (n PolicyTargetNames) contains(ref resource.PolicyTargetRef) bool {
	_, exists := n.values[policyTargetKey{Kind: ref.Kind, ID: ref.Name}]
	return exists
}

// PolicyTargetResolver 解析策略允许引用的资源并返回展示名称
type PolicyTargetResolver struct {
	gateways GatewayLister
	routes   RouteLister
	callers  CallerLister
}

// NewCallerPolicyTargetResolver 创建只解析 Caller 的策略目标解析器
func NewCallerPolicyTargetResolver(callers CallerLister) *PolicyTargetResolver {
	return &PolicyTargetResolver{callers: callers}
}

// NewPolicyTargetResolver 创建策略作用目标解析器
func NewPolicyTargetResolver(gateways GatewayLister, routes RouteLister) *PolicyTargetResolver {
	return &PolicyTargetResolver{gateways: gateways, routes: routes}
}

// Resolve 校验策略作用目标并返回当前展示名称
func (r *PolicyTargetResolver) Resolve(ctx context.Context, refs []resource.PolicyTargetRef) (PolicyTargetNames, error) {
	names, err := r.DisplayNames(ctx, refs)
	if err != nil {
		return PolicyTargetNames{}, err
	}
	for _, ref := range refs {
		if names.contains(ref) {
			continue
		}
		switch ref.Kind {
		case resource.KindGateway:
			return PolicyTargetNames{}, NewRuleViolation(fmt.Sprintf("网关 %q 不存在", ref.Name))
		case resource.KindRoute:
			return PolicyTargetNames{}, NewRuleViolation(fmt.Sprintf("路由 %q 不存在", ref.Name))
		case resource.KindCaller:
			return PolicyTargetNames{}, NewRuleViolation(fmt.Sprintf("调用方 %q 不存在", ref.Name))
		default:
			return PolicyTargetNames{}, NewRuleViolation("策略作用目标类型不正确")
		}
	}
	return names, nil
}

// DisplayNames 返回当前存在的策略作用目标展示名称，缺失引用保留为空名称供状态页展示
func (r *PolicyTargetResolver) DisplayNames(ctx context.Context, refs []resource.PolicyTargetRef) (PolicyTargetNames, error) {
	names := PolicyTargetNames{values: make(map[policyTargetKey]string, len(refs))}
	targets := make(map[resource.Kind]map[string]struct{}, 3)
	for _, ref := range refs {
		ids := targets[ref.Kind]
		if ids == nil {
			ids = make(map[string]struct{})
			targets[ref.Kind] = ids
		}
		ids[ref.Name] = struct{}{}
	}
	if ids := targets[resource.KindGateway]; len(ids) > 0 {
		resolved := 0
		if err := VisitPages(ctx, r.gateways.ListPage, func(item resource.Gateway) (bool, error) {
			if _, ok := ids[item.Name]; ok {
				names.values[policyTargetKey{Kind: resource.KindGateway, ID: item.Name}] = item.Spec.DisplayName
				resolved++
			}
			return resolved == len(ids), nil
		}); err != nil {
			return PolicyTargetNames{}, err
		}
	}
	if ids := targets[resource.KindRoute]; len(ids) > 0 {
		resolved := 0
		if err := VisitPages(ctx, r.routes.ListPage, func(item resource.Route) (bool, error) {
			if _, ok := ids[item.Name]; ok {
				names.values[policyTargetKey{Kind: resource.KindRoute, ID: item.Name}] = item.Spec.DisplayName
				resolved++
			}
			return resolved == len(ids), nil
		}); err != nil {
			return PolicyTargetNames{}, err
		}
	}
	if ids := targets[resource.KindCaller]; len(ids) > 0 {
		resolved := 0
		if err := VisitPages(ctx, r.callers.ListPage, func(item resource.Caller) (bool, error) {
			if _, ok := ids[item.Name]; ok {
				names.values[policyTargetKey{Kind: resource.KindCaller, ID: item.Name}] = item.Spec.DisplayName
				resolved++
			}
			return resolved == len(ids), nil
		}); err != nil {
			return PolicyTargetNames{}, err
		}
	}
	return names, nil
}
