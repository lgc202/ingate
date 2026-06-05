package service

import (
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	runtimegroupservice "github.com/lgc202/ingate/internal/adminapi/service/runtimegroup"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

// Service 聚合 admin-api 面向控制台的查询用例
type Service struct {
	Gateway      *gatewayservice.Service
	Route        *routeservice.Service
	RuntimeGroup *runtimegroupservice.Service
	Upstream     *upstreamservice.Service
}

// New 创建 service 聚合入口
func New(store *store.Store) *Service {
	runtimeGroupService := runtimegroupservice.New(store.RuntimeGroup)
	return &Service{
		Gateway:      gatewayservice.New(store.Gateway, store.Route, runtimeGroupService),
		Route:        routeservice.New(store.Route, store.Gateway, store.Upstream),
		RuntimeGroup: runtimeGroupService,
		Upstream:     upstreamservice.New(store.Upstream, store.Route),
	}
}
