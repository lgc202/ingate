package store

import (
	accesscontrolpolicystore "github.com/lgc202/ingate/internal/adminapi/store/accesscontrolpolicy"
	certificatestore "github.com/lgc202/ingate/internal/adminapi/store/certificate"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/adminapi/store/ratelimitpolicy"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 聚合 admin-api 访问 ingate-apiserver 的 store
type Store struct {
	Certificate         *certificatestore.Store
	Gateway             *gatewaystore.Store
	Route               *routestore.Store
	Upstream            *upstreamstore.Store
	AccessControlPolicy *accesscontrolpolicystore.Store
	RateLimitPolicy     *ratelimitpolicystore.Store
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
	}
}
