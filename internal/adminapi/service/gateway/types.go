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
	DisplayName  string
	Description  string
	RuntimeGroup string
	Listeners    []ListenerParams
	HostBindings []HostBindingParams
}

// UpdateGatewayParams 是更新 Gateway 用例参数
type UpdateGatewayParams struct {
	Version      string
	DisplayName  string
	Description  string
	RuntimeGroup string
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
