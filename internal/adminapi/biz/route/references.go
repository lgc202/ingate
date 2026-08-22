package route

import (
	"context"
	"fmt"
	"slices"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func (s *Service) ensureNotReferenced(ctx context.Context, route *resource.Route) error {
	usage, err := s.policyUsage.FindTarget(ctx, resource.PolicyTargetRef{Kind: resource.KindRoute, Name: route.Name})
	if err != nil {
		return err
	}
	if usage != nil {
		return biz.NewRuleViolation(fmt.Sprintf("路由 %q 仍被策略 %q 应用", route.Spec.DisplayName, usage.DisplayName))
	}
	return biz.VisitPages(ctx, s.callers.ListPage, func(caller resource.Caller) (bool, error) {
		if slices.Contains(caller.Spec.RouteRefs, route.Name) {
			return true, biz.NewRuleViolation(fmt.Sprintf("路由 %q 仍授权给调用方 %q", route.Spec.DisplayName, caller.Spec.DisplayName))
		}
		return false, nil
	})
}
