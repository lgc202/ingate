package caller

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// checkAuthorizedRoutes 预检 Caller 授权的 Route 是否存在且要求调用方身份。
// 最终鉴权行为仍由 Authz 根据当前已同步的 Caller 资源执行。
func (uc *Usecase) checkAuthorizedRoutes(ctx context.Context, routeIDs []string) error {
	routes, err := uc.routes.ListByIDs(ctx, routeIDs)
	if err != nil {
		return err
	}
	for _, routeID := range routeIDs {
		route := routes[routeID]
		if route == nil {
			return errors.Conflict(
				adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
				fmt.Sprintf("授权路由 %q 不存在", routeID),
			)
		}
		if route.Spec.AccessMode != resource.RouteAccessCaller {
			return errors.Conflict(
				adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
				fmt.Sprintf("路由 %q 不使用调用方密钥", route.Spec.DisplayName),
			)
		}
	}
	return nil
}

// checkNotReferenced 检查删除请求开始时可见的 Token 额度策略引用。
// 并发写入产生的悬空引用由 AI ExtProc 在执行边界忽略。
func (uc *Usecase) checkNotReferenced(ctx context.Context, caller *resource.Caller) error {
	return biz.VisitPages(
		ctx,
		uc.tokenQuotaPolicies.ListPage,
		func(policy resource.TokenQuotaPolicy) (bool, error) {
			for _, target := range policy.Spec.TargetRefs {
				if target.Kind == resource.KindCaller && target.Name == caller.Name {
					return false, errors.Conflict(
						adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
						fmt.Sprintf(
							"调用方 %q 仍被 Token 额度策略 %q 应用",
							caller.Spec.DisplayName,
							policy.Spec.DisplayName,
						),
					)
				}
			}
			return false, nil
		},
	)
}
