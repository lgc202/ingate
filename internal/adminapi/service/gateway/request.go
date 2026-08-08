package gateway

import (
	"fmt"
	"slices"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	standaloneHTTPPort  = 8080
	standaloneHTTPSPort = 8443
)

// buildGatewaySpec 校验控制台输入并构造声明式 Gateway 配置
func buildGatewaySpec(
	name string,
	description string,
	inputListeners []*adminv1.GatewayListener,
	inputHostnames []string,
) (resource.GatewaySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.GatewaySpec{}, adminservice.BadRequest("网关名称不能为空")
	}
	listeners, err := buildGatewayListeners(inputListeners)
	if err != nil {
		return resource.GatewaySpec{}, err
	}
	bindings, err := buildGatewayHostBindings(inputHostnames, listeners)
	if err != nil {
		return resource.GatewaySpec{}, err
	}
	return resource.GatewaySpec{
		DisplayName:  name,
		Description:  strings.TrimSpace(description),
		Listeners:    listeners,
		HostBindings: bindings,
	}, nil
}

func buildGatewayListeners(inputListeners []*adminv1.GatewayListener) ([]resource.Listener, error) {
	if len(inputListeners) == 0 {
		return nil, adminservice.BadRequest("至少需要启用一个运行入口")
	}
	listeners := make([]resource.Listener, 0, len(inputListeners))
	seenProtocols := make(map[resource.Protocol]struct{}, len(inputListeners))
	for _, input := range inputListeners {
		if input == nil {
			return nil, adminservice.BadRequest("运行入口配置不能为空")
		}
		protocol := resource.Protocol(input.GetProtocol())
		if _, exists := seenProtocols[protocol]; exists {
			return nil, adminservice.BadRequest("同一种运行入口协议只能配置一次")
		}
		seenProtocols[protocol] = struct{}{}

		certificateID := strings.TrimSpace(input.GetCertificateId())
		switch protocol {
		case resource.ProtocolHTTP:
			if input.GetPort() != standaloneHTTPPort {
				return nil, adminservice.BadRequest("HTTP 运行入口端口必须为 8080")
			}
			if certificateID != "" {
				return nil, adminservice.BadRequest("HTTP 运行入口不能配置证书")
			}
		case resource.ProtocolHTTPS:
			if input.GetPort() != standaloneHTTPSPort {
				return nil, adminservice.BadRequest("HTTPS 运行入口端口必须为 8443")
			}
			if certificateID == "" {
				return nil, adminservice.BadRequest("HTTPS 运行入口必须选择证书")
			}
		default:
			return nil, adminservice.BadRequest("运行入口协议不正确")
		}

		listeners = append(listeners, resource.Listener{
			Name:           strings.ToLower(string(protocol)),
			Protocol:       protocol,
			Port:           int(input.GetPort()),
			CertificateRef: certificateID,
		})
	}
	slices.SortFunc(listeners, func(a, b resource.Listener) int {
		return strings.Compare(string(a.Protocol), string(b.Protocol))
	})
	return listeners, nil
}

func buildGatewayHostBindings(inputHostnames []string, listeners []resource.Listener) ([]resource.HostBinding, error) {
	hostnames := make([]string, 0, len(inputHostnames))
	for _, value := range inputHostnames {
		hostname, ok := hostnameutil.Normalize(strings.ToLower(strings.TrimSpace(value)))
		if !ok || hostname == "*" {
			return nil, adminservice.BadRequest("网关域名格式不正确")
		}
		if slices.Contains(hostnames, hostname) {
			continue
		}
		for _, existing := range hostnames {
			if hostnameutil.Overlaps(hostname, existing) {
				return nil, adminservice.BadRequest(fmt.Sprintf("网关域名 %q 与 %q 的范围重叠", hostname, existing))
			}
		}
		hostnames = append(hostnames, hostname)
	}

	listenerRefs := make([]string, 0, len(listeners))
	for _, listener := range listeners {
		listenerRefs = append(listenerRefs, listener.Name)
	}
	bindings := make([]resource.HostBinding, 0, len(hostnames))
	for _, hostname := range hostnames {
		bindings = append(bindings, resource.HostBinding{
			Hostname:     hostname,
			ListenerRefs: append([]string(nil), listenerRefs...),
		})
	}
	return bindings, nil
}
