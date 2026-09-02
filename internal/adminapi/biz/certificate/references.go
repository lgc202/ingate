package certificate

import (
	"context"
	"fmt"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// checkNotReferenced 检查删除请求开始时可见的 Gateway 引用。
// 并发写入产生的悬空引用由引用方的 Controller Status 表达。
func (uc *Usecase) checkNotReferenced(ctx context.Context, certificate *resource.Certificate) error {
	return pagination.VisitPages(ctx, uc.gateways.ListPage, func(gateway resource.Gateway) (bool, error) {
		for _, listener := range gateway.Spec.Listeners {
			if listener.CertificateRef == certificate.Name {
				return false, adminv1.ErrorResourceReferenced("%s", fmt.Sprintf(
					"证书 %q 仍被网关 %q 引用",
					certificate.Spec.DisplayName,
					gateway.Spec.DisplayName,
				),
				)
			}
		}
		return false, nil
	})
}
