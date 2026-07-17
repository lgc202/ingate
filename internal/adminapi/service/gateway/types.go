package gateway

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 是 Gateway 列表用例结果
type ListResult struct {
	Gateways []resource.Gateway
}

// GatewayResult 是单个 Gateway 用例结果
type GatewayResult struct {
	Gateway *resource.Gateway
}

// CreateGatewayParams 是创建 Gateway 用例参数
type CreateGatewayParams struct {
	GatewayParams
}

// UpdateGatewayParams 是更新 Gateway 用例参数
type UpdateGatewayParams struct {
	Version string
	GatewayParams
}

// GatewayParams 是创建和更新 Gateway 共用的配置参数
type GatewayParams struct {
	Name         string
	Description  string
	Listeners    []ListenerParams
	HostBindings []HostBindingParams
}

// ListenerParams 是监听器用例参数
type ListenerParams struct {
	Name     string
	Protocol resource.Protocol
	Port     int
}

// HostBindingParams 是域名绑定用例参数
type HostBindingParams struct {
	Hostname       string
	ListenerRefs   []string
	CertificateRef string
}
