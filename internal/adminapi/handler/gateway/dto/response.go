package dto

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"github.com/samber/lo"
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
	return Gateway{
		ID:      gateway.Name,
		Version: gateway.ResourceVersion,
		GatewayConfig: GatewayConfig{
			Name:         gateway.Spec.DisplayName,
			Description:  gateway.Spec.Description,
			RuntimeGroup: gateway.Spec.RuntimeGroupRef.Name,
			Listeners:    listeners(gateway.Spec.Listeners),
			HostBindings: hostBindings(gateway.Spec.HostBindings),
		},
		Enabled:   gateway.Spec.Enabled,
		CreatedAt: createdAt(gateway.ObjectMeta),
	}
}

func listeners(items []resource.Listener) []GatewayListener {
	return lo.Map(items, func(item resource.Listener, _ int) GatewayListener {
		return GatewayListener{
			Name:     item.Name,
			Protocol: string(item.Protocol),
			Port:     item.Port,
		}
	})
}

func hostBindings(items []resource.HostBinding) []GatewayHostBinding {
	return lo.Map(items, func(item resource.HostBinding, _ int) GatewayHostBinding {
		binding := GatewayHostBinding{
			Hostname:     item.Hostname,
			ListenerRefs: append([]string(nil), item.ListenerRefs...),
		}
		if item.TLS != nil {
			binding.TLS = &GatewayTLS{CertificateRef: item.TLS.CertificateRef}
		}
		return binding
	})
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
