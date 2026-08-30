package biz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// GatewayLister 定义策略目标批量解析所需的 Gateway 查询能力。
type GatewayLister interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[resource.Gateway], error)
}

// RouteLister 定义策略目标批量解析所需的 Route 查询能力。
type RouteLister interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[resource.Route], error)
}

// CallerLister 定义调用方策略目标批量解析所需的查询能力。
type CallerLister interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[resource.Caller], error)
}

type policyTargetKey struct {
	kind resource.Kind
	id   string
}

// PolicyTargetNames 保存策略作用目标的展示名称。
type PolicyTargetNames struct {
	values map[policyTargetKey]string
}

// PolicyTargetResolver 解析策略允许引用的资源并返回展示名称。
type PolicyTargetResolver struct {
	gateways GatewayLister
	routes   RouteLister
	callers  CallerLister
}

// NewPolicyTargetResolver 创建 Gateway 和 Route 策略目标解析器。
func NewPolicyTargetResolver(gateways GatewayLister, routes RouteLister) *PolicyTargetResolver {
	return &PolicyTargetResolver{gateways: gateways, routes: routes}
}

// NewRoutePolicyTargetResolver 创建 Route 策略目标解析器。
func NewRoutePolicyTargetResolver(routes RouteLister) *PolicyTargetResolver {
	return &PolicyTargetResolver{routes: routes}
}

// NewCallerPolicyTargetResolver 创建 Caller 策略目标解析器。
func NewCallerPolicyTargetResolver(callers CallerLister) *PolicyTargetResolver {
	return &PolicyTargetResolver{callers: callers}
}

// Name 返回目标引用对应的展示名称。
func (n PolicyTargetNames) Name(ref resource.PolicyTargetRef) string {
	return n.values[policyTargetKey{kind: ref.Kind, id: ref.Name}]
}

// Resolve 校验策略作用目标并返回当前展示名称。
func (r *PolicyTargetResolver) Resolve(
	ctx context.Context,
	refs []resource.PolicyTargetRef,
) (PolicyTargetNames, error) {
	names, err := r.DisplayNames(ctx, refs)
	if err != nil {
		return PolicyTargetNames{}, err
	}
	for _, ref := range refs {
		key := policyTargetKey{kind: ref.Kind, id: ref.Name}
		if _, exists := names.values[key]; exists {
			continue
		}
		switch ref.Kind {
		case resource.KindGateway:
			if r.gateways == nil {
				return PolicyTargetNames{}, fmt.Errorf("resolve policy target: %s is not supported", ref.Kind)
			}
			return PolicyTargetNames{}, errors.Conflict(
				adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
				fmt.Sprintf("关联网关 %q 不存在", ref.Name),
			)
		case resource.KindRoute:
			if r.routes == nil {
				return PolicyTargetNames{}, fmt.Errorf("resolve policy target: %s is not supported", ref.Kind)
			}
			return PolicyTargetNames{}, errors.Conflict(
				adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
				fmt.Sprintf("关联路由 %q 不存在", ref.Name),
			)
		case resource.KindCaller:
			if r.callers == nil {
				return PolicyTargetNames{}, fmt.Errorf("resolve policy target: %s is not supported", ref.Kind)
			}
			return PolicyTargetNames{}, errors.Conflict(
				adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
				fmt.Sprintf("关联调用方 %q 不存在", ref.Name),
			)
		default:
			return PolicyTargetNames{}, fmt.Errorf("resolve policy target: unsupported kind %q", ref.Kind)
		}
	}
	return names, nil
}

// DisplayNames 返回当前存在的策略作用目标展示名称。
// 缺失引用保留为空名称供状态页展示。
func (r *PolicyTargetResolver) DisplayNames(
	ctx context.Context,
	refs []resource.PolicyTargetRef,
) (PolicyTargetNames, error) {
	names := PolicyTargetNames{values: make(map[policyTargetKey]string, len(refs))}
	targets := make(map[resource.Kind]map[string]bool, 3)
	for _, ref := range refs {
		ids := targets[ref.Kind]
		if ids == nil {
			ids = make(map[string]bool)
			targets[ref.Kind] = ids
		}
		ids[ref.Name] = true
	}
	if ids := targets[resource.KindGateway]; len(ids) > 0 && r.gateways != nil {
		resolved := 0
		if err := VisitPages(ctx, r.gateways.ListPage, func(item resource.Gateway) (bool, error) {
			if ids[item.Name] {
				names.values[policyTargetKey{kind: resource.KindGateway, id: item.Name}] = item.Spec.DisplayName
				resolved++
			}
			return resolved == len(ids), nil
		}); err != nil {
			return PolicyTargetNames{}, err
		}
	}
	if ids := targets[resource.KindRoute]; len(ids) > 0 && r.routes != nil {
		resolved := 0
		if err := VisitPages(ctx, r.routes.ListPage, func(item resource.Route) (bool, error) {
			if ids[item.Name] {
				names.values[policyTargetKey{kind: resource.KindRoute, id: item.Name}] = item.Spec.DisplayName
				resolved++
			}
			return resolved == len(ids), nil
		}); err != nil {
			return PolicyTargetNames{}, err
		}
	}
	if ids := targets[resource.KindCaller]; len(ids) > 0 && r.callers != nil {
		resolved := 0
		if err := VisitPages(ctx, r.callers.ListPage, func(item resource.Caller) (bool, error) {
			if ids[item.Name] {
				names.values[policyTargetKey{kind: resource.KindCaller, id: item.Name}] = item.Spec.DisplayName
				resolved++
			}
			return resolved == len(ids), nil
		}); err != nil {
			return PolicyTargetNames{}, err
		}
	}
	return names, nil
}
