package caller

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func (s *Service) ensureDisplayNameAvailable(ctx context.Context, callerID, displayName string) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(caller resource.Caller) (bool, error) {
		if caller.Name != callerID && caller.Spec.DisplayName == displayName {
			return true, biz.NewRuleViolation(fmt.Sprintf("调用方名称 %q 已存在", displayName))
		}
		return false, nil
	})
}

func (s *Service) validateAuthorizedRoutes(ctx context.Context, routeIDs []string) error {
	for _, routeID := range routeIDs {
		route, err := s.routes.Get(ctx, routeID)
		if err != nil {
			if errors.Is(err, biz.ErrResourceNotFound) {
				return biz.NewRuleViolation(fmt.Sprintf("授权路由 %q 不存在", routeID))
			}
			return err
		}
		if route.Spec.AccessMode != resource.RouteAccessCaller {
			return biz.NewRuleViolation(fmt.Sprintf("路由 %q 不使用调用方密钥", route.Spec.DisplayName))
		}
	}
	return nil
}
