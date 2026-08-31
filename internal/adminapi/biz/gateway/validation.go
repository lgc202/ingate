package gateway

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
)

// checkListenerClaimsAvailable 预检启用 Gateway 的 Listener 占用，以便立即返回明确错误。
// 声明式 API 的并发写入仍由 Controller status 最终裁决。
func (uc *Usecase) checkListenerClaimsAvailable(
	ctx context.Context,
	excludedGatewayID string,
	spec resource.GatewaySpec,
) error {
	if !spec.Enabled {
		return nil
	}

	return biz.VisitPages(ctx, uc.store.ListPage, func(candidate resource.Gateway) (bool, error) {
		if candidate.Name == excludedGatewayID || !candidate.Spec.Enabled {
			return false, nil
		}
		if err := checkListenerClaims(spec, candidate); err != nil {
			return false, err
		}
		return false, nil
	})
}

func checkListenerClaims(submittedSpec resource.GatewaySpec, existingGateway resource.Gateway) error {
	for _, submitted := range submittedSpec.Listeners {
		for _, existing := range existingGateway.Spec.Listeners {
			if submitted.Port != existing.Port {
				continue
			}
			if submitted.Protocol != existing.Protocol {
				return errors.Conflict(
					adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
					fmt.Sprintf(
						"端口 %d 已被网关 %q 的 %s 入口占用，不能同时配置为 %s 入口",
						submitted.Port,
						existingGateway.Spec.DisplayName,
						existing.Protocol,
						submitted.Protocol,
					),
				)
			}
			submittedHostname := listenerHostname(submitted)
			existingHostname := listenerHostname(existing)
			if hostnameutil.Overlaps(submittedHostname, existingHostname) {
				return errors.Conflict(
					adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
					fmt.Sprintf(
						"访问入口 %s:%d 的域名范围 %s 与网关 %q 的域名范围 %s 重叠；请调整域名，或先停用该网关",
						submitted.Protocol,
						submitted.Port,
						describeHostnameClaim(submittedHostname),
						existingGateway.Spec.DisplayName,
						describeHostnameClaim(existingHostname),
					),
				)
			}
		}
	}
	return nil
}

func listenerHostname(listener resource.Listener) string {
	if listener.Hostname == "" {
		return "*"
	}
	return listener.Hostname
}

func describeHostnameClaim(hostname string) string {
	if hostname == "*" {
		return "全部域名"
	}
	return fmt.Sprintf("%q", hostname)
}
