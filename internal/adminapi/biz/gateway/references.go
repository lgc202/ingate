package gateway

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func (s *Service) validateCertificateRefs(ctx context.Context, spec resource.GatewaySpec) error {
	seen := make(map[string]struct{}, len(spec.Listeners))
	for _, listener := range spec.Listeners {
		if listener.Protocol != resource.ProtocolHTTPS {
			continue
		}
		if _, exists := seen[listener.CertificateRef]; exists {
			continue
		}
		seen[listener.CertificateRef] = struct{}{}

		_, err := s.certificates.Get(ctx, listener.CertificateRef)
		if err != nil {
			if errors.Is(err, biz.ErrResourceNotFound) {
				return biz.NewRuleViolation(fmt.Sprintf("HTTPS 证书 %q 不存在", listener.CertificateRef))
			}
			return err
		}
	}
	return nil
}

func (s *Service) ensureNotReferenced(ctx context.Context, gateway *resource.Gateway) error {
	if err := biz.VisitPages(ctx, s.routes.ListPage, func(route resource.Route) (bool, error) {
		if slices.Contains(route.Spec.GatewayRefs, gateway.Name) {
			return true, biz.NewRuleViolation(fmt.Sprintf("网关 %q 仍有关联路由", gateway.Spec.DisplayName))
		}
		return false, nil
	}); err != nil {
		return err
	}

	usage, err := s.policyUsage.FindTarget(ctx, resource.PolicyTargetRef{Kind: resource.KindGateway, Name: gateway.Name})
	if err != nil {
		return err
	}
	if usage != nil {
		return biz.NewRuleViolation(fmt.Sprintf("网关 %q 仍被策略 %q 应用", gateway.Spec.DisplayName, usage.DisplayName))
	}
	return nil
}
