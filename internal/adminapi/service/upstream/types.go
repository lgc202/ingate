package upstream

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 是服务列表用例结果
type ListResult struct {
	Upstreams []resource.Upstream
	Routes    []resource.Route
}

// UpstreamResult 是单个服务用例结果
type UpstreamResult struct {
	Upstream *resource.Upstream
	Routes   []resource.Route
}
