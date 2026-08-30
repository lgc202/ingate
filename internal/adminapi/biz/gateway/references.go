package gateway

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// checkCertificateReferences 预检 HTTPS Listener 引用的证书。
// 最终发布结果仍由 Controller status 表达。
func (uc *Usecase) checkCertificateReferences(ctx context.Context, spec resource.GatewaySpec) error {
	seenCertificateIDs := make(map[string]bool, len(spec.Listeners))
	for _, listener := range spec.Listeners {
		if listener.Protocol != resource.ProtocolHTTPS {
			continue
		}
		certificateID := listener.CertificateRef
		if seenCertificateIDs[certificateID] {
			continue
		}
		seenCertificateIDs[certificateID] = true

		_, err := uc.certificates.Get(ctx, certificateID)
		if err != nil {
			if errors.IsNotFound(err) {
				return errors.Conflict(
					adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
					fmt.Sprintf("HTTPS 证书 %q 不存在", certificateID),
				).WithCause(err)
			}
			return err
		}
	}
	return nil
}

// checkNotReferenced 检查删除请求开始时可见的引用。
// 并发写入产生的悬空引用由引用方的 Controller Status 表达。
func (uc *Usecase) checkNotReferenced(ctx context.Context, gateway *resource.Gateway) error {
	if err := biz.VisitPages(ctx, uc.routes.ListPage, func(route resource.Route) (bool, error) {
		if slices.Contains(route.Spec.GatewayRefs, gateway.Name) {
			return false, errors.Conflict(
				adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
				fmt.Sprintf("网关 %q 仍有关联路由", gateway.Spec.DisplayName),
			)
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
		return errors.Conflict(
			adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
			fmt.Sprintf(
				"网关 %q 仍被策略 %q 应用",
				gateway.Spec.DisplayName,
				policyUsage.DisplayName,
			),
		)
	}
	return nil
}
