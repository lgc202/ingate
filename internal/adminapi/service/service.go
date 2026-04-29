package service

import (
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	runtimeservice "github.com/lgc202/ingate/internal/adminapi/service/runtime"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

// Service 聚合 admin-api 面向控制台的查询用例
type Service struct {
	Gateway  *gatewayservice.Service
	Route    *routeservice.Service
	Runtime  *runtimeservice.Service
	Upstream *upstreamservice.Service
}

// New 创建 service 聚合入口
func New(store *store.Store) *Service {
	return &Service{
		Gateway:  gatewayservice.New(store.Gateway),
		Route:    routeservice.New(store.Route),
		Runtime:  runtimeservice.New(store.Runtime),
		Upstream: upstreamservice.New(store.Upstream),
	}
}
