package gateway

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
)

func (s *Service) ensureListenerClaimsAvailable(ctx context.Context, gatewayID string, spec resource.GatewaySpec) error {
	if !spec.Enabled {
		return nil
	}

	// 这里为控制台提供立即冲突提示；声明式 API 并发写入仍以 Controller status 为最终裁决
	return biz.VisitPages(ctx, s.repository.ListPage, func(current resource.Gateway) (bool, error) {
		if current.Name == gatewayID || !current.Spec.Enabled {
			return false, nil
		}
		if err := validateListenerClaims(spec, current); err != nil {
			return true, err
		}
		return false, nil
	})
}

func validateListenerClaims(submitted resource.GatewaySpec, current resource.Gateway) error {
	for _, listener := range submitted.Listeners {
		for _, currentListener := range current.Spec.Listeners {
			if listener.Port != currentListener.Port {
				continue
			}
			if listener.Protocol != currentListener.Protocol {
				return biz.NewRuleViolation(fmt.Sprintf(
					"端口 %d 已被网关 %q 的 %s 入口占用，不能同时配置为 %s 入口",
					listener.Port,
					current.Spec.DisplayName,
					currentListener.Protocol,
					listener.Protocol,
				))
			}
			hostname := listenerHostname(listener)
			currentHostname := listenerHostname(currentListener)
			if hostnameutil.Overlaps(hostname, currentHostname) {
				return biz.NewRuleViolation(fmt.Sprintf(
					"访问入口 %s:%d 的域名范围 %s 与网关 %q 的域名范围 %s 重叠；请调整域名，或先停用该网关",
					listener.Protocol,
					listener.Port,
					hostClaimDescription(hostname),
					current.Spec.DisplayName,
					hostClaimDescription(currentHostname),
				))
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

func hostClaimDescription(hostname string) string {
	if hostname == "*" {
		return "全部域名"
	}
	return fmt.Sprintf("%q", hostname)
}
