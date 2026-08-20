package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func (s *Service) validateGateway(ctx context.Context, spec resource.GatewaySpec, excludeID string) error {
	if err := s.validateCertificateRefs(ctx, spec); err != nil {
		return err
	}
	if !spec.Enabled {
		return nil
	}

	// 这里为控制台提供立即冲突提示；声明式 API 并发写入仍以 Controller status 为最终裁决
	return biz.VisitPages(ctx, s.repository.ListPage, func(current resource.Gateway) (bool, error) {
		if current.Name == excludeID || !current.Spec.Enabled {
			return false, nil
		}
		if err := validateListenerClaims(spec, current); err != nil {
			return true, err
		}
		return false, nil
	})
}

func (s *Service) validateCertificateRefs(ctx context.Context, spec resource.GatewaySpec) error {
	seen := make(map[string]struct{})
	for _, listener := range spec.Listeners {
		if listener.Protocol != resource.ProtocolHTTPS {
			continue
		}
		if _, exists := seen[listener.CertificateRef]; exists {
			continue
		}
		seen[listener.CertificateRef] = struct{}{}

		_, err := s.certificates.Get(ctx, listener.CertificateRef)
		if err == nil {
			continue
		}
		if errors.Is(err, biz.ErrResourceNotFound) {
			return biz.NewUserError(fmt.Sprintf("HTTPS 证书 %q 不存在", listener.CertificateRef))
		}
		return err
	}
	return nil
}

func validateListenerClaims(submitted resource.GatewaySpec, current resource.Gateway) error {
	for _, listener := range submitted.Listeners {
		for _, currentListener := range current.Spec.Listeners {
			if listener.Port != currentListener.Port {
				continue
			}
			if listener.Protocol != currentListener.Protocol {
				return biz.NewUserError(fmt.Sprintf(
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
				return biz.NewUserError(fmt.Sprintf(
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
