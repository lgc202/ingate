package route

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// WorkspaceResult 是路由页面工作区用例结果
type WorkspaceResult struct {
	Routes    []resource.Route
	Gateways  []resource.Gateway
	Upstreams []resource.Upstream
}

// RouteResult 是单个路由用例结果
type RouteResult struct {
	Route *resource.Route
}
