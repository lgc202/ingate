package upstream

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func (s *Service) ensureNotReferenced(ctx context.Context, upstream *resource.Upstream) error {
	return biz.VisitPages(ctx, s.routes.ListPage, func(route resource.Route) (bool, error) {
		if routeReferencesUpstream(route.Spec, upstream.Name) {
			return true, biz.NewRuleViolation(fmt.Sprintf(
				"服务 %q 仍被路由 %q 引用",
				upstream.Spec.DisplayName,
				routeDisplayName(route),
			))
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
