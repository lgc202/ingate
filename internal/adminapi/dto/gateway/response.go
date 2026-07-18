package gateway

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewListGatewaysResp 转换 Gateway 列表用例结果为 HTTP 响应
func NewListGatewaysResp(result *gatewayservice.ListResult) ListGatewaysResp {
	gateways := make([]Gateway, 0, len(result.Gateways))
	for i := range result.Gateways {
		gateways = append(gateways, gatewayFromResource(&result.Gateways[i]))
	}
	return ListGatewaysResp{Gateways: gateways}
}

// NewGetGatewayResp 转换单个 Gateway 用例结果为 HTTP 响应
func NewGetGatewayResp(result *gatewayservice.GatewayResult) GetGatewayResp {
	return GetGatewayResp{
		Gateway: gatewayFromResource(result.Gateway),
	}
}

func gatewayFromResource(gateway *resource.Gateway) Gateway {
	status := admindto.NewResourceStatus(gateway.Generation, gateway.Status.Conditions)
	if !gateway.Spec.Enabled && admindto.ConfigurationApplied(gateway.Generation, gateway.Status.Conditions) {
		status = admindto.NewDisabledResourceStatus()
	}
	return Gateway{
		ID:      gateway.Name,
		Version: gateway.ResourceVersion,
		Status:  status,
		GatewayConfig: GatewayConfig{
			Name:        gateway.Spec.DisplayName,
			Description: gateway.Spec.Description,
			Listeners:   listeners(gateway.Spec.Listeners),
			Hostnames:   hostnames(gateway.Spec.HostBindings),
		},
		Enabled:   gateway.Spec.Enabled,
		CreatedAt: createdAt(gateway.ObjectMeta),
	}
}

func listeners(items []resource.Listener) []GatewayListener {
	listeners := make([]GatewayListener, 0, len(items))
	for _, listener := range items {
		listeners = append(listeners, GatewayListener{
			Protocol:      listener.Protocol,
			Port:          listener.Port,
			CertificateID: listener.CertificateRef,
		})
	}
	return listeners
}

func hostnames(bindings []resource.HostBinding) []string {
	hostnames := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Hostname != "" {
			hostnames = append(hostnames, binding.Hostname)
		}
	}
	return hostnames
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
