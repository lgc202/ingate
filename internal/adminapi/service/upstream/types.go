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
}

// UpdateUpstreamParams 是更新 Upstream 用例参数
type UpdateUpstreamParams struct {
	Version string
	UpstreamParams
}

// UpstreamParams 是创建和更新 Upstream 共用的配置参数
type UpstreamParams struct {
	Name              string
	Type              resource.UpstreamType
	LoadBalancePolicy resource.UpstreamLoadBalancePolicy
	Endpoints         []EndpointParams
	HealthCheck       *resource.UpstreamHealthCheck
}

// EndpointParams 是 Upstream 端点配置参数
type EndpointParams struct {
	ID      string
	Address string
	Port    int
	Weight  int
	Enabled bool
}
