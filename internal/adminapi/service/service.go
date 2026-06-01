package service

import (
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

// Service 聚合 admin-api 面向控制台的查询用例
type Service struct {
	Gateway  *gatewayservice.Service
	Route    *routeservice.Service
	Upstream *upstreamservice.Service
}

// New 创建 service 聚合入口
func New(store *store.Store) *Service {
	return &Service{
		Gateway:  gatewayservice.New(store.Gateway, store.Route, store.Runtime, store.Upstream),
		Route:    routeservice.New(store.Route, store.Gateway, store.Upstream),
		Upstream: upstreamservice.New(store.Upstream, store.Route),
	}
}
