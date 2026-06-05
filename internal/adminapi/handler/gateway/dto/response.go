package dto

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	runtimegroupsvc "github.com/lgc202/ingate/internal/adminapi/service/runtimegroup"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewListGatewaysResp 转换 Gateway 列表用例结果为 HTTP 响应
func NewListGatewaysResp(result *gatewayservice.ListResult) ListGatewaysResp {
	gateways := make([]GatewaySummary, 0, len(result.Gateways))
	for i := range result.Gateways {
		gateways = append(gateways, gatewaySummary(&result.Gateways[i]))
	}
	return ListGatewaysResp{Gateways: gateways}
}

// NewGetGatewayResp 转换单个 Gateway 用例结果为 HTTP 响应
func NewGetGatewayResp(result *gatewayservice.GatewayResult) GetGatewayResp {
	return GetGatewayResp{
		Gateway: gatewayDetail(result.Gateway),
	}
}

func gatewaySummary(gateway *resource.Gateway) GatewaySummary {
	return GatewaySummary{
		ID:                 gateway.Name,
		Version:            gateway.ResourceVersion,
		DisplayName:        gateway.Spec.DisplayName,
		Description:        gateway.Spec.Description,
		RuntimeGroup:       runtimeGroup(gateway),
		ListenerSummary:    listenerSummary(gateway.Spec.Listeners),
		HostBindingSummary: hostBindingSummary(gateway.Spec.HostBindings),
		Listeners:          listeners(gateway.Spec.Listeners),
		HostBindings:       hostBindings(gateway.Spec.HostBindings),
		Enabled:            gateway.Spec.Enabled,
		HealthStatus:       healthStatus(gateway.Status),
		LastChangedAt:      lastChangedAt(gateway.ObjectMeta),
	}
}

func gatewayDetail(gateway *resource.Gateway) GatewayDetail {
	return GatewayDetail{
		ID:            gateway.Name,
		Version:       gateway.ResourceVersion,
		DisplayName:   gateway.Spec.DisplayName,
		Description:   gateway.Spec.Description,
		RuntimeGroup:  runtimeGroup(gateway),
		Listeners:     listeners(gateway.Spec.Listeners),
		HostBindings:  hostBindings(gateway.Spec.HostBindings),
		Enabled:       gateway.Spec.Enabled,
		HealthStatus:  healthStatus(gateway.Status),
		LastChangedAt: lastChangedAt(gateway.ObjectMeta),
	}
}

func runtimeGroup(gateway *resource.Gateway) string {
	if gateway.Spec.RuntimeGroupRef.Name == "" {
		return runtimegroupsvc.DefaultID
	}
	return gateway.Spec.RuntimeGroupRef.Name
}

func listeners(items []resource.Listener) []GatewayListener {
	listeners := make([]GatewayListener, 0, len(items))
	for _, item := range items {
		listeners = append(listeners, GatewayListener{
			Name:     item.Name,
			Protocol: string(item.Protocol),
			Port:     item.Port,
		})
	}
	return listeners
}

func hostBindings(items []resource.HostBinding) []GatewayHostBinding {
	bindings := make([]GatewayHostBinding, 0, len(items))
	for _, item := range items {
		binding := GatewayHostBinding{
			Hostname:     item.Hostname,
			ListenerRefs: append([]string(nil), item.ListenerRefs...),
		}
		if item.TLS != nil {
			binding.TLS = &GatewayTLS{CertificateRef: item.TLS.CertificateRef}
		}
		bindings = append(bindings, binding)
	}
	return bindings
}

func listenerSummary(listeners []resource.Listener) string {
	if len(listeners) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(listeners))
	for _, listener := range listeners {
		parts = append(parts, fmt.Sprintf("%s:%d", listener.Protocol, listener.Port))
	}
	return strings.Join(parts, " / ")
}

func hostBindingSummary(bindings []resource.HostBinding) string {
	if len(bindings) == 0 {
		return "不限制 Host"
	}
	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Hostname == "" {
			parts = append(parts, "不限制 Host")
			continue
		}
		parts = append(parts, binding.Hostname)
	}
	return strings.Join(parts, "、")
}

func healthStatus(status resource.ResourceStatus) string {
	for _, condition := range status.Conditions {
		if condition.Type == "Ready" && condition.Status == metav1.ConditionFalse {
			return "critical"
		}
		if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
			return "healthy"
		}
	}
	return "unknown"
}

func lastChangedAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
