package service

import (
	accesscontrolpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/accesscontrolpolicy"
	certificateservice "github.com/lgc202/ingate/internal/adminapi/service/certificate"
	configurationstatusservice "github.com/lgc202/ingate/internal/adminapi/service/configurationstatus"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	ratelimitpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimitpolicy"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

// Service 聚合 admin-api 面向控制台的查询用例
type Service struct {
	Certificate         *certificateservice.Service
	ConfigurationStatus *configurationstatusservice.Service
	Gateway             *gatewayservice.Service
	Route               *routeservice.Service
	Upstream            *upstreamservice.Service
	AccessControlPolicy *accesscontrolpolicyservice.Service
	RateLimitPolicy     *ratelimitpolicyservice.Service
}

// New 创建 service 聚合入口
func New(store *store.Store) *Service {
	policyUsage := policytarget.NewUsageFinder(store.RateLimitPolicy, store.AccessControlPolicy)
	configurationStatus := configurationstatusservice.New(
		store.Gateway,
		store.Route,
		store.Upstream,
		store.Certificate,
		store.RateLimitPolicy,
		store.AccessControlPolicy,
	)
	return &Service{
		Certificate:         certificateservice.New(store.Certificate, store.Gateway),
		ConfigurationStatus: configurationStatus,
		Gateway:             gatewayservice.New(store.Gateway, store.Route, store.Certificate, policyUsage),
		Route:               routeservice.New(store.Route, store.Gateway, store.Upstream, policyUsage),
		Upstream:            upstreamservice.New(store.Upstream, store.Route),
		AccessControlPolicy: accesscontrolpolicyservice.New(store.AccessControlPolicy, store.Gateway, store.Route),
		RateLimitPolicy:     ratelimitpolicyservice.New(store.RateLimitPolicy, store.Gateway, store.Route),
	}
}
