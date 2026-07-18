package upstream

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 是服务列表用例结果
type ListResult struct {
	Upstreams []resource.Upstream
}

// UpstreamResult 是单个服务用例结果
type UpstreamResult struct {
	Upstream *resource.Upstream
}

// CreateUpstreamParams 是创建 Upstream 用例参数
type CreateUpstreamParams struct {
	UpstreamParams
	APIKey *APIKeyParams
}

// UpdateUpstreamParams 是更新 Upstream 用例参数
type UpdateUpstreamParams struct {
	Version      string
	APIKey       *APIKeyParams
	RemoveAPIKey bool
	UpstreamParams
}

// UpstreamParams 是创建和更新 Upstream 共用的配置参数
type UpstreamParams struct {
	Name              string
	Type              resource.UpstreamType
	Protocol          resource.UpstreamProtocol
	TLS               *TLSParams
	LoadBalancePolicy resource.UpstreamLoadBalancePolicy
	Endpoints         []EndpointParams
	HealthCheck       *resource.UpstreamHealthCheck
}

// APIKeyParams 是 Upstream 使用的 API Key 参数
type APIKeyParams struct {
	Value string
}

// TLSParams 是 Upstream HTTPS 连接参数
type TLSParams struct {
	ServerName string
}

// EndpointParams 是 Upstream 端点配置参数
type EndpointParams struct {
	ID      string
	Address string
	Port    int
	Weight  int
	Enabled bool
}
