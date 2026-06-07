package store

import (
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	policybindingstore "github.com/lgc202/ingate/internal/adminapi/store/policybinding"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/adminapi/store/ratelimitpolicy"
	redisstorestore "github.com/lgc202/ingate/internal/adminapi/store/redisstore"
	resourcestore "github.com/lgc202/ingate/internal/adminapi/store/resource"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	runtimestore "github.com/lgc202/ingate/internal/adminapi/store/runtime"
	runtimegroupstore "github.com/lgc202/ingate/internal/adminapi/store/runtimegroup"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 聚合 admin-api 访问 ingate-apiserver 的 store
type Store struct {
	Gateway         *gatewaystore.Store
	Route           *routestore.Store
	Runtime         *runtimestore.Store
	RuntimeGroup    *runtimegroupstore.Store
	Upstream        *upstreamstore.Store
	RateLimitPolicy *ratelimitpolicystore.Store
	PolicyBinding   *policybindingstore.Store
	RedisStore      *redisstorestore.Store
	Resource        *resourcestore.Store
}

// New 创建 store 聚合入口
func New(client clientset.Interface) *Store {
	return &Store{
		Gateway:         gatewaystore.New(client),
		Route:           routestore.New(client),
		Runtime:         runtimestore.New(client),
		RuntimeGroup:    runtimegroupstore.New(client),
		Upstream:        upstreamstore.New(client),
		RateLimitPolicy: ratelimitpolicystore.New(client),
		PolicyBinding:   policybindingstore.New(client),
		RedisStore:      redisstorestore.New(client),
		Resource:        resourcestore.New(client),
	}
}
