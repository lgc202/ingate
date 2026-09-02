// Package policy 提供策略目标、状态和引用关系的共享领域能力。
package policy

import (
	"context"
	"fmt"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// GatewayReader 定义策略目标解析所需的 Gateway 批量读取能力。
type GatewayReader interface {
	ListByIDs(ctx context.Context, gatewayIDs []string) (map[string]*resource.Gateway, error)
}

// RouteReader 定义策略目标解析所需的 Route 批量读取能力。
type RouteReader interface {
	ListByIDs(ctx context.Context, routeIDs []string) (map[string]*resource.Route, error)
}

// CallerReader 定义策略目标解析所需的 Caller 批量读取能力。
type CallerReader interface {
	ListByIDs(ctx context.Context, callerIDs []string) (map[string]*resource.Caller, error)
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
	gateways GatewayReader
	routes   RouteReader
	callers  CallerReader
}

// NewPolicyTargetResolver 创建 Gateway 和 Route 策略目标解析器。
func NewPolicyTargetResolver(gateways GatewayReader, routes RouteReader) *PolicyTargetResolver {
	return &PolicyTargetResolver{gateways: gateways, routes: routes}
}

// NewRoutePolicyTargetResolver 创建 Route 策略目标解析器。
func NewRoutePolicyTargetResolver(routes RouteReader) *PolicyTargetResolver {
	return &PolicyTargetResolver{routes: routes}
}

// NewCallerPolicyTargetResolver 创建 Caller 策略目标解析器。
func NewCallerPolicyTargetResolver(callers CallerReader) *PolicyTargetResolver {
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
			return PolicyTargetNames{}, adminv1.ErrorPolicyTargetNotFound("%s", fmt.Sprintf("关联网关 %q 不存在", ref.Name))
		case resource.KindRoute:
			if r.routes == nil {
				return PolicyTargetNames{}, fmt.Errorf("resolve policy target: %s is not supported", ref.Kind)
			}
			return PolicyTargetNames{}, adminv1.ErrorPolicyTargetNotFound("%s", fmt.Sprintf("关联路由 %q 不存在", ref.Name))
		case resource.KindCaller:
			if r.callers == nil {
				return PolicyTargetNames{}, fmt.Errorf("resolve policy target: %s is not supported", ref.Kind)
			}
			return PolicyTargetNames{}, adminv1.ErrorPolicyTargetNotFound("%s", fmt.Sprintf("关联调用方 %q 不存在", ref.Name))
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
	targetIDs := make(map[resource.Kind][]string, 3)
	seen := make(map[policyTargetKey]bool, len(refs))
	for _, ref := range refs {
		key := policyTargetKey{kind: ref.Kind, id: ref.Name}
		if seen[key] {
			continue
		}
		seen[key] = true
		targetIDs[ref.Kind] = append(targetIDs[ref.Kind], ref.Name)
	}
	if ids := targetIDs[resource.KindGateway]; len(ids) > 0 && r.gateways != nil {
		gateways, err := r.gateways.ListByIDs(ctx, ids)
		if err != nil {
			return PolicyTargetNames{}, err
		}
		for id, gateway := range gateways {
			names.values[policyTargetKey{kind: resource.KindGateway, id: id}] = gateway.Spec.DisplayName
		}
	}
	if ids := targetIDs[resource.KindRoute]; len(ids) > 0 && r.routes != nil {
		routes, err := r.routes.ListByIDs(ctx, ids)
		if err != nil {
			return PolicyTargetNames{}, err
		}
		for id, route := range routes {
			names.values[policyTargetKey{kind: resource.KindRoute, id: id}] = route.Spec.DisplayName
		}
	}
	if ids := targetIDs[resource.KindCaller]; len(ids) > 0 && r.callers != nil {
		callers, err := r.callers.ListByIDs(ctx, ids)
		if err != nil {
			return PolicyTargetNames{}, err
		}
		for id, caller := range callers {
			names.values[policyTargetKey{kind: resource.KindCaller, id: id}] = caller.Spec.DisplayName
		}
	}
	return names, nil
}
