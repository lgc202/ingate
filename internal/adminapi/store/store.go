package store

import (
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	resourcestore "github.com/lgc202/ingate/internal/adminapi/store/resource"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	runtimestore "github.com/lgc202/ingate/internal/adminapi/store/runtime"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 聚合 admin-api 访问 ingate-apiserver 的 store
type Store struct {
	Gateway  *gatewaystore.Store
	Resource *resourcestore.Store
	Route    *routestore.Store
	Runtime  *runtimestore.Store
	Upstream *upstreamstore.Store
}

// New 创建 store 聚合入口
func New(client clientset.Interface) *Store {
	return &Store{
		Gateway:  gatewaystore.New(client),
		Resource: resourcestore.New(client),
		Route:    routestore.New(client),
		Runtime:  runtimestore.New(client),
		Upstream: upstreamstore.New(client),
	}
}
