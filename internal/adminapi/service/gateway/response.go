package gateway

import (
	"strconv"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func newGatewayReply(gateway *resource.Gateway) *adminv1.Gateway {
	status := biz.EnabledResourceStatus(gateway.Generation, gateway.Spec.Enabled, gateway.Status.Conditions)
	listeners := make([]*adminv1.GatewayListener, 0, len(gateway.Spec.Listeners))
	for _, listener := range gateway.Spec.Listeners {
		listeners = append(listeners, &adminv1.GatewayListener{
			Protocol:      string(listener.Protocol),
			Port:          int32(listener.Port),
			CertificateId: listener.CertificateRef,
		})
	}
	hostnames := make([]string, 0, len(gateway.Spec.HostBindings))
	for _, binding := range gateway.Spec.HostBindings {
		if binding.Hostname != "" {
			hostnames = append(hostnames, binding.Hostname)
		}
	}
	return &adminv1.Gateway{
		Id:          gateway.Name,
		Version:     strconv.FormatInt(gateway.Generation, 10),
		Status:      adminservice.NewResourceStatus(status),
		Name:        gateway.Spec.DisplayName,
		Description: gateway.Spec.Description,
		Listeners:   listeners,
		Hostnames:   hostnames,
		Enabled:     gateway.Spec.Enabled,
		CreatedAt:   adminservice.NewTimestamp(gateway.CreationTimestamp.Time),
	}
}
