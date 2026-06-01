package gateway

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 是 Gateway 列表用例的聚合结果，仍保持后端资源语义，不作为 HTTP 响应直接返回。
type ListResult struct {
	Gateways         []resource.Gateway
	Routes           []resource.Route
	Upstreams        []resource.Upstream
	RuntimeSnapshots []resource.RuntimeSnapshot
}

// GatewayResult 是单个 Gateway 用例的聚合结果。
type GatewayResult struct {
	Gateway          *resource.Gateway
	Routes           []resource.Route
	Upstreams        []resource.Upstream
	RuntimeSnapshots []resource.RuntimeSnapshot
}

// DetailResult 是 Gateway 详情用例的聚合结果。
type DetailResult struct {
	Gateway          *resource.Gateway
	Routes           []resource.Route
	Upstreams        []resource.Upstream
	RuntimeSnapshots []resource.RuntimeSnapshot
}
