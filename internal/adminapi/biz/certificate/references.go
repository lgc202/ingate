package certificate

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func (s *Service) ensureNotReferenced(ctx context.Context, certificate *resource.Certificate) error {
	return biz.VisitPages(ctx, s.gateways.ListPage, func(gateway resource.Gateway) (bool, error) {
		for _, listener := range gateway.Spec.Listeners {
			if listener.CertificateRef == certificate.Name {
				return true, biz.NewRuleViolation(fmt.Sprintf(
					"证书 %q 仍被网关 %q 引用",
					certificate.Spec.DisplayName,
					gateway.Spec.DisplayName,
				))
			}
		}
		return false, nil
	})
}
