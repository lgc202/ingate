package route

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// checkReferences 预检 Route 引用的资源，以便写入请求立即返回明确错误。
// 最终发布结果仍由 Controller status 表达。
func (uc *Usecase) checkReferences(ctx context.Context, spec resource.RouteSpec) error {
	gateways, err := uc.gateways.ListByIDs(ctx, spec.GatewayRefs)
	if err != nil {
		return err
	}
	for _, gatewayID := range spec.GatewayRefs {
		if gateways[gatewayID] == nil {
			return errors.Conflict(
				adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
				fmt.Sprintf("关联网关 %q 不存在", gatewayID),
			)
		}
	}

	serviceIDs := referencedServiceIDs(spec)
	services, err := uc.services.ListByIDs(ctx, serviceIDs)
	if err != nil {
		return err
	}
	for _, serviceRef := range spec.UpstreamRefs {
		service := services[serviceRef.Name]
		if service == nil {
			return errors.Conflict(
				adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
				fmt.Sprintf("关联服务 %q 不存在", serviceRef.Name),
			)
		}
		if service.Spec.Model != nil {
			return errors.Conflict(
				adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
				fmt.Sprintf("模型服务 %q 只能用于 AI 路由", service.Spec.DisplayName),
			)
		}
	}

	if spec.AI == nil {
		return nil
	}

	for _, model := range spec.AI.Models {
		for _, target := range model.Targets {
			service := services[target.UpstreamRef]
			if service == nil {
				return errors.Conflict(
					adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
					fmt.Sprintf("关联模型服务 %q 不存在", target.UpstreamRef),
				)
			}
			if service.Spec.Model == nil {
				return errors.Conflict(
					adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
					fmt.Sprintf("服务 %q 不是模型服务", service.Spec.DisplayName),
				)
			}
		}
	}

	return nil
}

func referencedServiceIDs(spec resource.RouteSpec) []string {
	serviceCount := len(spec.UpstreamRefs)
	if spec.AI != nil {
		for _, model := range spec.AI.Models {
			serviceCount += len(model.Targets)
		}
	}

	serviceIDs := make([]string, 0, serviceCount)
	for _, serviceRef := range spec.UpstreamRefs {
		serviceIDs = append(serviceIDs, serviceRef.Name)
	}
	if spec.AI != nil {
		for _, model := range spec.AI.Models {
			for _, target := range model.Targets {
				serviceIDs = append(serviceIDs, target.UpstreamRef)
			}
		}
	}
	return serviceIDs
}

// checkNotReferenced 检查删除请求开始时可见的引用。
// 并发写入产生的悬空引用由引用方的 Controller Status 表达。
func (uc *Usecase) checkNotReferenced(ctx context.Context, route *resource.Route) error {
	targetRef := resource.PolicyTargetRef{
		Kind: resource.KindRoute,
		Name: route.Name,
	}
	policyUsage, err := uc.policyUsageFinder.FindTarget(ctx, targetRef)
	if err != nil {
		return err
	}
	if policyUsage != nil {
		return errors.Conflict(
			adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
			fmt.Sprintf(
				"路由 %q 仍被策略 %q 应用",
				route.Spec.DisplayName,
				policyUsage.DisplayName,
			),
		)
	}

	return biz.VisitPages(
		ctx,
		uc.callers.ListPage,
		func(caller resource.Caller) (bool, error) {
			if slices.Contains(caller.Spec.RouteRefs, route.Name) {
				return false, errors.Conflict(
					adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
					fmt.Sprintf(
						"路由 %q 仍授权给调用方 %q",
						route.Spec.DisplayName,
						caller.Spec.DisplayName,
					),
				)
			}
			return false, nil
		},
	)
}
