package store

import (
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	runtimestore "github.com/lgc202/ingate/internal/adminapi/store/runtime"
	runtimegroupstore "github.com/lgc202/ingate/internal/adminapi/store/runtimegroup"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 聚合 admin-api 访问 ingate-apiserver 的 store
type Store struct {
	Gateway      *gatewaystore.Store
	Route        *routestore.Store
	Runtime      *runtimestore.Store
	RuntimeGroup *runtimegroupstore.Store
	Upstream     *upstreamstore.Store
}

// New 创建 store 聚合入口
func New(client clientset.Interface) *Store {
	return &Store{
		Gateway:      gatewaystore.New(client),
		Route:        routestore.New(client),
		Runtime:      runtimestore.New(client),
		RuntimeGroup: runtimegroupstore.New(client),
		Upstream:     upstreamstore.New(client),
	}
}
