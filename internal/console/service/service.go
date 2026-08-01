package service

import (
	accesscontrolpolicyservice "github.com/lgc202/ingate/internal/console/service/accesscontrolpolicy"
	certificateservice "github.com/lgc202/ingate/internal/console/service/certificate"
	configurationstatusservice "github.com/lgc202/ingate/internal/console/service/configurationstatus"
	gatewayservice "github.com/lgc202/ingate/internal/console/service/gateway"
	"github.com/lgc202/ingate/internal/console/service/policytarget"
	ratelimitpolicyservice "github.com/lgc202/ingate/internal/console/service/ratelimitpolicy"
	routeservice "github.com/lgc202/ingate/internal/console/service/route"
	tokenquotapolicyservice "github.com/lgc202/ingate/internal/console/service/tokenquotapolicy"
	upstreamservice "github.com/lgc202/ingate/internal/console/service/upstream"
	"github.com/lgc202/ingate/internal/console/store"
)

// Service 聚合 Console 面向用户的管理用例
type Service struct {
	Certificate         *certificateservice.Service
	ConfigurationStatus *configurationstatusservice.Service
	Gateway             *gatewayservice.Service
	Route               *routeservice.Service
	Upstream            *upstreamservice.Service
	AccessControlPolicy *accesscontrolpolicyservice.Service
	RateLimitPolicy     *ratelimitpolicyservice.Service
	TokenQuotaPolicy    *tokenquotapolicyservice.Service
}

// New 创建 service 聚合入口
func New(store *store.Store) *Service {
	policyUsage := policytarget.NewUsageFinder(store.RateLimitPolicy, store.AccessControlPolicy, store.TokenQuotaPolicy)
	configurationStatus := configurationstatusservice.New(
		store.Gateway,
		store.Route,
		store.Upstream,
		store.Certificate,
		store.RateLimitPolicy,
		store.AccessControlPolicy,
		store.TokenQuotaPolicy,
	)
	return &Service{
		Certificate:         certificateservice.New(store.Certificate, store.Gateway),
		ConfigurationStatus: configurationStatus,
		Gateway:             gatewayservice.New(store.Gateway, store.Route, store.Certificate, policyUsage),
		Route:               routeservice.New(store.Route, store.Gateway, store.Upstream, policyUsage),
		Upstream:            upstreamservice.New(store.Upstream, store.Route),
		AccessControlPolicy: accesscontrolpolicyservice.New(store.AccessControlPolicy, store.Gateway, store.Route),
		RateLimitPolicy:     ratelimitpolicyservice.New(store.RateLimitPolicy, store.Gateway, store.Route),
		TokenQuotaPolicy:    tokenquotapolicyservice.New(store.TokenQuotaPolicy, store.Gateway, store.Route),
	}
}
