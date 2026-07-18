// Package policytarget 负责策略作用目标的存在性校验和展示名称解析
package policytarget

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Key 唯一标识一个策略作用目标
type Key struct {
	Kind resource.Kind
	ID   string
}

// DisplayNames 保存策略作用目标的展示名称
type DisplayNames map[Key]string

// Name 返回目标引用对应的展示名称
func (n DisplayNames) Name(ref resource.PolicyTargetRef) string {
	return n[Key{Kind: ref.Kind, ID: ref.Name}]
}

// Contains 判断目标引用当前是否存在
func (n DisplayNames) Contains(ref resource.PolicyTargetRef) bool {
	_, exists := n[Key{Kind: ref.Kind, ID: ref.Name}]
	return exists
}

// Resolver 解析 Gateway 和 Route 策略作用目标
type Resolver struct {
	gateways *gatewaystore.Store
	routes   *routestore.Store
}

// New 创建策略作用目标解析器
func New(gateways *gatewaystore.Store, routes *routestore.Store) *Resolver {
	return &Resolver{gateways: gateways, routes: routes}
}

// Validate 校验所有策略作用目标是否存在
func (r *Resolver) Validate(ctx context.Context, refs []resource.PolicyTargetRef) error {
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
			return xerrors.NewUserError(fmt.Sprintf("网关 %q 不存在", ref.Name))
		case resource.KindRoute:
			return xerrors.NewUserError(fmt.Sprintf("路由 %q 不存在", ref.Name))
		default:
			return xerrors.NewUserError("策略作用目标只支持网关或路由")
		}
	}
	return nil
}

// DisplayNames 返回当前存在的策略作用目标展示名称，缺失引用保留为空名称供状态页展示
func (r *Resolver) DisplayNames(ctx context.Context, refs []resource.PolicyTargetRef) (DisplayNames, error) {
	names := make(DisplayNames)
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
		for _, gateway := range gateways.Items {
			names[Key{Kind: resource.KindGateway, ID: gateway.Name}] = gateway.Spec.DisplayName
		}
	}
	if needRoutes {
		routes, err := r.routes.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, route := range routes.Items {
			names[Key{Kind: resource.KindRoute, ID: route.Name}] = route.Spec.DisplayName
		}
	}
	return names, nil
}
