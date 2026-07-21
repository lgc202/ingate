package gateway

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// Spec 将已校验的创建请求转换为声明式 GatewaySpec
func (r CreateGatewayReq) Spec() resource.GatewaySpec {
	return r.GatewayConfig.spec()
}

// Spec 将已校验的更新请求转换为声明式 GatewaySpec
func (r UpdateGatewayReq) Spec() resource.GatewaySpec {
	return r.GatewayConfig.spec()
}

func (c GatewayConfig) spec() resource.GatewaySpec {
	listeners := make([]resource.Listener, 0, len(c.Listeners))
	listenerRefs := make([]string, 0, len(c.Listeners))
	for _, listener := range c.Listeners {
		name := listenerName(listener.Protocol)
		listeners = append(listeners, resource.Listener{
			Name:           name,
			Protocol:       listener.Protocol,
			Port:           listener.Port,
			CertificateRef: listener.CertificateID,
		})
		listenerRefs = append(listenerRefs, name)
	}

	var hostBindings []resource.HostBinding
	for _, hostname := range c.Hostnames {
		hostBindings = append(hostBindings, resource.HostBinding{
			Hostname:     hostname,
			ListenerRefs: append([]string(nil), listenerRefs...),
		})
	}

	return resource.GatewaySpec{
		DisplayName:  c.Name,
		Description:  c.Description,
		Listeners:    listeners,
		HostBindings: hostBindings,
	}
}

func listenerName(protocol resource.Protocol) string {
	if protocol == resource.ProtocolHTTPS {
		return "https"
	}
	return "http"
}
