package gateway

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func gatewayResponse(gateway *resource.Gateway) *adminv1.Gateway {
	status := biz.EnabledResourceStatus(
		gateway.Generation,
		gateway.Spec.Enabled,
		gateway.Status.Conditions,
	)
	listeners := make([]*adminv1.GatewayListener, len(gateway.Spec.Listeners))
	for i, listener := range gateway.Spec.Listeners {
		listeners[i] = &adminv1.GatewayListener{
			Name:          listener.Name,
			Protocol:      gatewayProtocolResponse(listener.Protocol),
			Port:          uint32(listener.Port),
			Hostname:      listener.Hostname,
			CertificateId: listener.CertificateRef,
		}
	}

	return &adminv1.Gateway{
		Id:        gateway.Name,
		Name:      gateway.Spec.DisplayName,
		Enabled:   gateway.Spec.Enabled,
		Listeners: listeners,
		State:     adminservice.ResourceState(status.State),
		Message:   adminservice.ResourceMessage(status.Reason),
		Version:   gateway.Generation,
		CreatedAt: adminservice.Timestamp(gateway.CreationTimestamp.Time),
		UpdatedAt: adminservice.Timestamp(adminservice.ResourceUpdatedAt(gateway.Annotations)),
	}
}

func gatewayProtocolResponse(protocol resource.Protocol) adminv1.GatewayProtocol {
	switch protocol {
	case resource.ProtocolHTTP:
		return adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTP
	case resource.ProtocolHTTPS:
		return adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTPS
	default:
		return adminv1.GatewayProtocol_GATEWAY_PROTOCOL_UNSPECIFIED
	}
}
