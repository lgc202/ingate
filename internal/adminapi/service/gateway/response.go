package gateway

import (
	"time"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func gatewayFromResource(gateway *resource.Gateway) *adminv1.Gateway {
	status := biz.EnabledResourceStatus(gateway.Generation, gateway.Spec.Enabled, gateway.Status.Conditions)
	listeners := make([]*adminv1.GatewayListener, 0, len(gateway.Spec.Listeners))
	for _, listener := range gateway.Spec.Listeners {
		listeners = append(listeners, &adminv1.GatewayListener{
			Name:          listener.Name,
			Protocol:      gatewayProtocolFromResource(listener.Protocol),
			Port:          uint32(listener.Port),
			Hostname:      listener.Hostname,
			CertificateId: listener.CertificateRef,
		})
	}

	return &adminv1.Gateway{
		Id:        gateway.Name,
		Name:      gateway.Spec.DisplayName,
		Enabled:   gateway.Spec.Enabled,
		Listeners: listeners,
		State:     adminservice.NewResourceState(status.State),
		Message:   adminservice.ResourceMessage(status.Reason),
		Version:   gateway.Generation,
		CreatedAt: adminservice.NewTimestamp(gateway.CreationTimestamp.Time),
		UpdatedAt: adminservice.NewTimestamp(updatedAt(gateway)),
	}
}

func gatewayProtocolFromResource(protocol resource.Protocol) adminv1.GatewayProtocol {
	switch protocol {
	case resource.ProtocolHTTP:
		return adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTP
	case resource.ProtocolHTTPS:
		return adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTPS
	default:
		return adminv1.GatewayProtocol_GATEWAY_PROTOCOL_UNSPECIFIED
	}
}

func updatedAt(gateway *resource.Gateway) time.Time {
	value := gateway.Annotations[resource.AnnotationUpdatedAt]
	if value == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
