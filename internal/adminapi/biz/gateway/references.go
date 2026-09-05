package gateway

import (
	"context"
	"fmt"
	"slices"

	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// checkCertificateReferences 预检 HTTPS Listener 引用的证书。
// 最终发布结果仍由 Controller status 表达。
func (uc *Usecase) checkCertificateReferences(ctx context.Context, spec resource.GatewaySpec) error {
	certificateIDs := lo.FilterMap(spec.Listeners, func(listener resource.Listener, _ int) (string, bool) {
		return listener.CertificateRef, listener.Protocol == resource.ProtocolHTTPS
	})
	certificates, err := uc.certificates.ListByIDs(ctx, certificateIDs)
	if err != nil {
		return err
	}
	for _, certificateID := range certificateIDs {
		if certificates[certificateID] != nil {
			continue
		}
		return adminv1.ErrorResourceReferenceNotFound("%s", fmt.Sprintf("HTTPS 证书 %q 不存在", certificateID))
	}
	return nil
}

// checkNotReferenced 检查删除请求开始时可见的引用。
// 并发写入产生的悬空引用由引用方的 Controller Status 表达。
func (uc *Usecase) checkNotReferenced(ctx context.Context, gateway *resource.Gateway) error {
	if err := pagination.VisitPages(ctx, uc.routes.ListPage, func(route resource.Route) (bool, error) {
		if slices.Contains(route.Spec.GatewayRefs, gateway.Name) {
			return false, adminv1.ErrorResourceReferenced("%s", fmt.Sprintf("网关 %q 仍有关联路由", gateway.Spec.DisplayName))
		}
		return false, nil
	}); err != nil {
		return err
	}

	targetRef := resource.PolicyTargetRef{
		Kind: resource.KindGateway,
		Name: gateway.Name,
	}
	policyUsage, err := uc.policyUsageFinder.FindTarget(ctx, targetRef)
	if err != nil {
		return err
	}
	if policyUsage != nil {
		return adminv1.ErrorResourceReferenced("%s", fmt.Sprintf(
			"网关 %q 仍被策略 %q 应用",
			gateway.Spec.DisplayName,
			policyUsage.DisplayName,
		),
		)
	}
	return nil
}
