package upstream

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// checkNotReferenced 检查删除请求开始时可见的 Route 引用。
// 并发写入产生的悬空引用由引用方的 Controller Status 表达。
func (uc *Usecase) checkNotReferenced(ctx context.Context, upstream *resource.Upstream) error {
	return biz.VisitPages(ctx, uc.routes.ListPage, func(route resource.Route) (bool, error) {
		if routeReferencesUpstream(route.Spec, upstream.Name) {
			return false, errors.Conflict(
				adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
				fmt.Sprintf(
					"服务 %q 仍被路由 %q 引用",
					upstream.Spec.DisplayName,
					routeDisplayName(route),
				),
			)
		}
		return false, nil
	})
}

func routeReferencesUpstream(spec resource.RouteSpec, upstreamID string) bool {
	for _, ref := range spec.UpstreamRefs {
		if ref.Name == upstreamID {
			return true
		}
	}
	if spec.AI == nil {
		return false
	}
	for _, model := range spec.AI.Models {
		for _, target := range model.Targets {
			if target.UpstreamRef == upstreamID {
				return true
			}
		}
	}
	return false
}

func routeDisplayName(route resource.Route) string {
	if route.Spec.DisplayName != "" {
		return route.Spec.DisplayName
	}
	return route.Name
}
