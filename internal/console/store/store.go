package store

import (
	accesscontrolpolicystore "github.com/lgc202/ingate/internal/console/store/accesscontrolpolicy"
	certificatestore "github.com/lgc202/ingate/internal/console/store/certificate"
	gatewaystore "github.com/lgc202/ingate/internal/console/store/gateway"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/console/store/ratelimitpolicy"
	routestore "github.com/lgc202/ingate/internal/console/store/route"
	tokenquotapolicystore "github.com/lgc202/ingate/internal/console/store/tokenquotapolicy"
	upstreamstore "github.com/lgc202/ingate/internal/console/store/upstream"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 聚合 Console 访问 ingate-apiserver 的资源存储
type Store struct {
	Certificate         *certificatestore.Store
	Gateway             *gatewaystore.Store
	Route               *routestore.Store
	Upstream            *upstreamstore.Store
	AccessControlPolicy *accesscontrolpolicystore.Store
	RateLimitPolicy     *ratelimitpolicystore.Store
	TokenQuotaPolicy    *tokenquotapolicystore.Store
}

// New 创建 store 聚合入口
func New(client clientset.Interface) *Store {
	return &Store{
		Certificate:         certificatestore.New(client),
		Gateway:             gatewaystore.New(client),
		Route:               routestore.New(client),
		Upstream:            upstreamstore.New(client),
		AccessControlPolicy: accesscontrolpolicystore.New(client),
		RateLimitPolicy:     ratelimitpolicystore.New(client),
		TokenQuotaPolicy:    tokenquotapolicystore.New(client),
	}
}
