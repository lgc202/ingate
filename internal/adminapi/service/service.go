package service

import (
	accesscontrolpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/accesscontrolpolicy"
	certificateservice "github.com/lgc202/ingate/internal/adminapi/service/certificate"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	policybindingservice "github.com/lgc202/ingate/internal/adminapi/service/policybinding"
	ratelimitpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimitpolicy"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

// Service 聚合 admin-api 面向控制台的查询用例
type Service struct {
	Certificate         *certificateservice.Service
	Gateway             *gatewayservice.Service
	Route               *routeservice.Service
	Upstream            *upstreamservice.Service
	AccessControlPolicy *accesscontrolpolicyservice.Service
	RateLimitPolicy     *ratelimitpolicyservice.Service
	PolicyBinding       *policybindingservice.Service
}

// New 创建 service 聚合入口
func New(store *store.Store) *Service {
	return &Service{
		Certificate:         certificateservice.New(store.Certificate, store.Gateway),
		Gateway:             gatewayservice.New(store.Gateway, store.Route, store.Certificate),
		Route:               routeservice.New(store.Route, store.Gateway, store.Upstream),
		Upstream:            upstreamservice.New(store.Upstream, store.Route),
		AccessControlPolicy: accesscontrolpolicyservice.New(store.AccessControlPolicy, store.PolicyBinding),
		RateLimitPolicy:     ratelimitpolicyservice.New(store.RateLimitPolicy, store.PolicyBinding),
		PolicyBinding:       policybindingservice.New(store.PolicyBinding, store.Gateway, store.Route, store.RateLimitPolicy, store.AccessControlPolicy),
	}
}
