package gateway

import (
	"fmt"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/gatewayconfig"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
)

func parseGatewaySpec(
	displayName string,
	enabled bool,
	listenerConfigs []*adminv1.GatewayListener,
) (resource.GatewaySpec, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return resource.GatewaySpec{}, adminv1.ErrorInvalidArgument("网关名称不能为空")
	}
	listeners, err := parseListeners(listenerConfigs)
	if err != nil {
		return resource.GatewaySpec{}, err
	}
	return resource.GatewaySpec{
		DisplayName: displayName,
		Enabled:     enabled,
		Listeners:   listeners,
	}, nil
}

func parseListeners(configs []*adminv1.GatewayListener) ([]resource.Listener, error) {
	if len(configs) == 0 {
		return nil, adminv1.ErrorInvalidArgument("至少需要配置一个监听入口")
	}
	if len(configs) > gatewayconfig.MaxListeners {
		return nil, adminv1.ErrorInvalidArgument("监听入口数量超过限制")
	}

	listeners := make([]resource.Listener, 0, len(configs))
	seenNames := make(map[string]bool, len(configs))
	for _, config := range configs {
		if config == nil {
			return nil, adminv1.ErrorInvalidArgument("监听入口不能为空")
		}
		listener, err := parseListener(config)
		if err != nil {
			return nil, err
		}
		if seenNames[listener.Name] {
			return nil, adminv1.ErrorInvalidArgument("%s", fmt.Sprintf("监听入口名称 %q 不能重复", listener.Name))
		}
		if err := checkListenerOverlap(listener, listeners); err != nil {
			return nil, err
		}
		seenNames[listener.Name] = true
		listeners = append(listeners, listener)
	}
	return listeners, nil
}

func parseListener(config *adminv1.GatewayListener) (resource.Listener, error) {
	listenerName := strings.TrimSpace(config.GetName())
	if !gatewayconfig.IsValidListenerName(listenerName) {
		return resource.Listener{}, adminv1.ErrorInvalidArgument("监听入口名称只能包含小写字母、数字和连字符，且必须以字母或数字开头和结尾")
	}

	protocol, err := parseProtocol(config.GetProtocol())
	if err != nil {
		return resource.Listener{}, err
	}
	port := int(config.GetPort())
	if !gatewayconfig.IsValidListenerPort(port) {
		return resource.Listener{}, adminv1.ErrorInvalidArgument("监听端口必须在 1 到 65535 之间")
	}

	hostnameValue := strings.ToLower(strings.TrimSpace(config.GetHostname()))
	hostname, ok := hostnameutil.Normalize(hostnameValue)
	if !ok || hostnameValue == "*" {
		return resource.Listener{}, adminv1.ErrorInvalidArgument("监听域名格式不正确，留空表示不限制域名")
	}
	if hostname == "*" {
		hostname = ""
	}

	certificateID := strings.TrimSpace(config.GetCertificateId())
	switch protocol {
	case resource.ProtocolHTTP:
		if certificateID != "" {
			return resource.Listener{}, adminv1.ErrorInvalidArgument("HTTP 监听入口不能配置证书")
		}
	case resource.ProtocolHTTPS:
		if certificateID == "" {
			return resource.Listener{}, adminv1.ErrorInvalidArgument("HTTPS 监听入口必须选择证书")
		}
	}

	return resource.Listener{
		Name:           listenerName,
		Protocol:       protocol,
		Port:           port,
		Hostname:       hostname,
		CertificateRef: certificateID,
	}, nil
}

func parseProtocol(protocol adminv1.GatewayProtocol) (resource.Protocol, error) {
	switch protocol {
	case adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTP:
		return resource.ProtocolHTTP, nil
	case adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTPS:
		return resource.ProtocolHTTPS, nil
	default:
		return "", adminv1.ErrorInvalidArgument("监听协议不正确")
	}
}

func checkListenerOverlap(listener resource.Listener, existing []resource.Listener) error {
	hostname := listener.Hostname
	if hostname == "" {
		hostname = "*"
	}
	for _, current := range existing {
		if listener.Port != current.Port {
			continue
		}
		if listener.Protocol != current.Protocol {
			return adminv1.ErrorInvalidArgument("%s", fmt.Sprintf("端口 %d 不能同时用于 HTTP 和 HTTPS", listener.Port))
		}
		currentHostname := current.Hostname
		if currentHostname == "" {
			currentHostname = "*"
		}
		if hostnameutil.Overlaps(hostname, currentHostname) {
			return adminv1.ErrorInvalidArgument("%s", fmt.Sprintf("端口 %d 上的监听域名范围不能重叠", listener.Port))
		}
	}
	return nil
}
