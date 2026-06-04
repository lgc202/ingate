package route

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 是 Route 列表查询结果
type ListResult struct {
	Routes []resource.Route
}

// RouteResult 是单个路由用例结果
type RouteResult struct {
	Route *resource.Route
}
