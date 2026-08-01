package gateway

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	standaloneHTTPPort  = 8080
	standaloneHTTPSPort = 8443
)

// Validate 校验创建 Gateway 请求
func (r *CreateGatewayReq) Validate() error {
	return r.GatewayConfig.Validate()
}

// Validate 校验更新 Gateway 请求
func (r *UpdateGatewayReq) Validate() error {
	if r.Version == "" {
		return errors.New("网关版本不能为空")
	}
	return r.GatewayConfig.Validate()
}

// Validate 校验 Gateway 配置请求
func (r *GatewayConfig) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.New("网关名称不能为空")
	}
	if len(r.Listeners) == 0 {
		return errors.New("至少需要启用一个运行入口")
	}

	seenProtocols := make(map[resource.Protocol]struct{}, len(r.Listeners))
	for i := range r.Listeners {
		listener := &r.Listeners[i]
		listener.CertificateID = strings.TrimSpace(listener.CertificateID)
		if _, exists := seenProtocols[listener.Protocol]; exists {
			return errors.New("同一种运行入口协议只能配置一次")
		}
		seenProtocols[listener.Protocol] = struct{}{}

		switch listener.Protocol {
		case resource.ProtocolHTTP:
			if listener.Port != standaloneHTTPPort {
				return errors.New("HTTP 运行入口端口必须为 8080")
			}
			if listener.CertificateID != "" {
				return errors.New("HTTP 运行入口不能配置证书")
			}
		case resource.ProtocolHTTPS:
			if listener.Port != standaloneHTTPSPort {
				return errors.New("HTTPS 运行入口端口必须为 8443")
			}
			if listener.CertificateID == "" {
				return errors.New("HTTPS 运行入口必须选择证书")
			}
		default:
			return errors.New("运行入口协议不正确")
		}
	}
	slices.SortFunc(r.Listeners, func(a, b GatewayListener) int {
		return strings.Compare(string(a.Protocol), string(b.Protocol))
	})

	seenHostnames := make(map[string]struct{}, len(r.Hostnames))
	hostnames := make([]string, 0, len(r.Hostnames))
	for _, value := range r.Hostnames {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return errors.New("网关域名不能为空")
		}
		hostname, ok := hostnameutil.Normalize(value)
		if !ok || hostname == "*" {
			return errors.New("网关域名格式不正确")
		}
		if _, exists := seenHostnames[hostname]; exists {
			continue
		}
		for _, existing := range hostnames {
			if hostnameutil.Overlaps(hostname, existing) {
				return fmt.Errorf("网关域名 %q 与 %q 的范围重叠", hostname, existing)
			}
		}
		seenHostnames[hostname] = struct{}{}
		hostnames = append(hostnames, hostname)
	}
	r.Hostnames = hostnames
	return nil
}

// Validate 校验 Gateway 启停请求
func (r *SetGatewayEnabledReq) Validate() error {
	if r.Enabled == nil {
		return errors.New("启用状态不能为空")
	}
	return nil
}

// Value 返回已校验的启停值
func (r *SetGatewayEnabledReq) Value() bool {
	return *r.Enabled
}
