package store

import (
	accesscontrolpolicystore "github.com/lgc202/ingate/internal/adminapi/store/accesscontrolpolicy"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	policybindingstore "github.com/lgc202/ingate/internal/adminapi/store/policybinding"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/adminapi/store/ratelimitpolicy"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 聚合 admin-api 访问 ingate-apiserver 的 store
type Store struct {
	Gateway             *gatewaystore.Store
	Route               *routestore.Store
	Upstream            *upstreamstore.Store
	AccessControlPolicy *accesscontrolpolicystore.Store
	RateLimitPolicy     *ratelimitpolicystore.Store
	PolicyBinding       *policybindingstore.Store
}

// New 创建 store 聚合入口
func New(client clientset.Interface) *Store {
	return &Store{
		Gateway:             gatewaystore.New(client),
		Route:               routestore.New(client),
		Upstream:            upstreamstore.New(client),
		AccessControlPolicy: accesscontrolpolicystore.New(client),
		RateLimitPolicy:     ratelimitpolicystore.New(client),
		PolicyBinding:       policybindingstore.New(client),
	}
}
