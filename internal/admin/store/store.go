package store

import (
	"database/sql"

	accesscontrolpolicystore "github.com/lgc202/ingate/internal/admin/store/accesscontrolpolicy"
	accesskeystore "github.com/lgc202/ingate/internal/admin/store/accesskey"
	certificatestore "github.com/lgc202/ingate/internal/admin/store/certificate"
	gatewaystore "github.com/lgc202/ingate/internal/admin/store/gateway"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/admin/store/ratelimitpolicy"
	routestore "github.com/lgc202/ingate/internal/admin/store/route"
	tokenquotapolicystore "github.com/lgc202/ingate/internal/admin/store/tokenquotapolicy"
	upstreamstore "github.com/lgc202/ingate/internal/admin/store/upstream"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 聚合 Admin 访问 ingate-apiserver 的资源存储
type Store struct {
	AccessKey           *accesskeystore.Store
	Certificate         *certificatestore.Store
	Gateway             *gatewaystore.Store
	Route               *routestore.Store
	Upstream            *upstreamstore.Store
	AccessControlPolicy *accesscontrolpolicystore.Store
	RateLimitPolicy     *ratelimitpolicystore.Store
	TokenQuotaPolicy    *tokenquotapolicystore.Store
}

// New 创建 store 聚合入口
func New(client clientset.Interface, database *sql.DB) *Store {
	return &Store{
		AccessKey:           accesskeystore.New(database),
		Certificate:         certificatestore.New(client),
		Gateway:             gatewaystore.New(client),
		Route:               routestore.New(client),
		Upstream:            upstreamstore.New(client),
		AccessControlPolicy: accesscontrolpolicystore.New(client),
		RateLimitPolicy:     ratelimitpolicystore.New(client),
		TokenQuotaPolicy:    tokenquotapolicystore.New(client),
	}
}
