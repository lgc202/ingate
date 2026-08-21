package gateway

import (
	"fmt"
	"strings"

	utilvalidation "k8s.io/apimachinery/pkg/util/validation"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
)

// createSpec 把创建请求转换为声明式 Gateway 配置
func createSpec(request *adminv1.CreateGatewayRequest) (resource.GatewaySpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.GatewaySpec{}, adminservice.BadRequest("网关名称不能为空")
	}
	listeners, err := gatewayListeners(request.GetListeners())
	if err != nil {
		return resource.GatewaySpec{}, err
	}
	return resource.GatewaySpec{
		DisplayName: name,
		Enabled:     request.GetEnabled(),
		Listeners:   listeners,
	}, nil
}

// updateSpec 把更新请求转换为声明式 Gateway 配置
func updateSpec(request *adminv1.UpdateGatewayRequest) (resource.GatewaySpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.GatewaySpec{}, adminservice.BadRequest("网关名称不能为空")
	}
	listeners, err := gatewayListeners(request.GetListeners())
	if err != nil {
		return resource.GatewaySpec{}, err
	}
	return resource.GatewaySpec{
		DisplayName: name,
		Enabled:     request.GetEnabled(),
		Listeners:   listeners,
	}, nil
}

func gatewayListeners(inputs []*adminv1.GatewayListener) ([]resource.Listener, error) {
	listeners := make([]resource.Listener, 0, len(inputs))
	names := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		if input == nil {
			return nil, adminservice.BadRequest("监听入口不能为空")
		}
		listener, err := gatewayListener(input)
		if err != nil {
			return nil, err
		}
		if _, exists := names[listener.Name]; exists {
			return nil, adminservice.BadRequest(fmt.Sprintf("监听入口名称 %q 不能重复", listener.Name))
		}
		if err := validateListenerOverlap(listener, listeners); err != nil {
			return nil, err
		}
		names[listener.Name] = struct{}{}
		listeners = append(listeners, listener)
	}
	return listeners, nil
}

func gatewayListener(input *adminv1.GatewayListener) (resource.Listener, error) {
	name := strings.TrimSpace(input.GetName())
	if name == "" {
		return resource.Listener{}, adminservice.BadRequest("监听入口名称不能为空")
	}
	if messages := utilvalidation.IsDNS1123Label(name); len(messages) > 0 {
		return resource.Listener{}, adminservice.BadRequest(
			"监听入口名称只能包含小写字母、数字和连字符，并且必须以字母或数字开头和结尾",
		)
	}

	protocol, err := gatewayProtocol(input.GetProtocol())
	if err != nil {
		return resource.Listener{}, err
	}
	port := int(input.GetPort())

	hostname := strings.ToLower(strings.TrimSpace(input.GetHostname()))
	normalizedHostname, hostnameValid := hostnameutil.Normalize(hostname)
	if !hostnameValid || hostname == "*" {
		return resource.Listener{}, adminservice.BadRequest("监听域名格式不正确，留空表示不限制域名")
	}
	if normalizedHostname == "*" {
		normalizedHostname = ""
	}

	certificateID := strings.TrimSpace(input.GetCertificateId())
	switch protocol {
	case resource.ProtocolHTTP:
		if certificateID != "" {
			return resource.Listener{}, adminservice.BadRequest("HTTP 监听入口不能配置证书")
		}
	case resource.ProtocolHTTPS:
		if certificateID == "" {
			return resource.Listener{}, adminservice.BadRequest("HTTPS 监听入口必须选择证书")
		}
	}

	return resource.Listener{
		Name:           name,
		Protocol:       protocol,
		Port:           port,
		Hostname:       normalizedHostname,
		CertificateRef: certificateID,
	}, nil
}

func gatewayProtocol(protocol adminv1.GatewayProtocol) (resource.Protocol, error) {
	switch protocol {
	case adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTP:
		return resource.ProtocolHTTP, nil
	case adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTPS:
		return resource.ProtocolHTTPS, nil
	default:
		return "", adminservice.BadRequest("监听协议不正确")
	}
}

func validateListenerOverlap(listener resource.Listener, existing []resource.Listener) error {
	hostname := listener.Hostname
	if hostname == "" {
		hostname = "*"
	}
	for _, current := range existing {
		if listener.Port != current.Port {
			continue
		}
		if listener.Protocol != current.Protocol {
			return adminservice.BadRequest(fmt.Sprintf("端口 %d 不能同时用于 HTTP 和 HTTPS", listener.Port))
		}
		currentHostname := current.Hostname
		if currentHostname == "" {
			currentHostname = "*"
		}
		if hostnameutil.Overlaps(hostname, currentHostname) {
			return adminservice.BadRequest(fmt.Sprintf("端口 %d 上的监听域名范围不能重叠", listener.Port))
		}
	}
	return nil
}
