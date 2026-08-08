// Package data 实现业务用例依赖的数据访问
package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/biz/accesscontrol"
	"github.com/lgc202/ingate/internal/adminapi/biz/accesskey"
	"github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	"github.com/lgc202/ingate/internal/adminapi/biz/configuration"
	"github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	"github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	"github.com/lgc202/ingate/internal/adminapi/biz/route"
	"github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	"github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/data/apiserver"
	"github.com/lgc202/ingate/internal/adminapi/data/cache"
	accesskeydao "github.com/lgc202/ingate/internal/adminapi/data/dao/accesskey"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	"github.com/lgc202/ingate/pkg/mysqlx"
	"github.com/lgc202/ingate/pkg/redisx"
)

const connectTimeout = 10 * time.Second

// ProviderSet 汇总 Admin API 的数据访问实现
var ProviderSet = wire.NewSet(
	NewData,
	NewResourceClient,
	NewDB,
	NewRedisClient,
	apiserver.NewGatewayRepository,
	apiserver.NewRouteRepository,
	apiserver.NewUpstreamRepository,
	apiserver.NewCertificateRepository,
	apiserver.NewRateLimitPolicyRepository,
	apiserver.NewAccessControlPolicyRepository,
	apiserver.NewTokenQuotaPolicyRepository,
	accesskeydao.NewDAO,
	cache.NewCredentialIndex,
	NewAccessKeyRepository,
	NewAccessKeyIndexSync,
	// 根 biz 只保留跨领域策略能力所需的只读边界
	wire.Bind(new(biz.GatewayLister), new(*apiserver.GatewayRepository)),
	wire.Bind(new(biz.RouteLister), new(*apiserver.RouteRepository)),
	wire.Bind(new(biz.RateLimitPolicyLister), new(*apiserver.RateLimitPolicyRepository)),
	wire.Bind(new(biz.AccessControlPolicyLister), new(*apiserver.AccessControlPolicyRepository)),
	wire.Bind(new(biz.TokenQuotaPolicyLister), new(*apiserver.TokenQuotaPolicyRepository)),
	// 每个领域声明自己真实消费的 Repository，避免 biz 子包相互依赖
	wire.Bind(new(gateway.Repository), new(*apiserver.GatewayRepository)),
	wire.Bind(new(gateway.RouteRepository), new(*apiserver.RouteRepository)),
	wire.Bind(new(gateway.CertificateRepository), new(*apiserver.CertificateRepository)),
	wire.Bind(new(route.Repository), new(*apiserver.RouteRepository)),
	wire.Bind(new(route.GatewayRepository), new(*apiserver.GatewayRepository)),
	wire.Bind(new(route.UpstreamRepository), new(*apiserver.UpstreamRepository)),
	wire.Bind(new(upstream.Repository), new(*apiserver.UpstreamRepository)),
	wire.Bind(new(upstream.RouteRepository), new(*apiserver.RouteRepository)),
	wire.Bind(new(certificate.Repository), new(*apiserver.CertificateRepository)),
	wire.Bind(new(certificate.GatewayRepository), new(*apiserver.GatewayRepository)),
	wire.Bind(new(ratelimit.Repository), new(*apiserver.RateLimitPolicyRepository)),
	wire.Bind(new(accesscontrol.Repository), new(*apiserver.AccessControlPolicyRepository)),
	wire.Bind(new(tokenquota.Repository), new(*apiserver.TokenQuotaPolicyRepository)),
	wire.Bind(new(accesskey.Repository), new(*accessKeyRepository)),
	wire.Bind(new(configuration.GatewayRepository), new(*apiserver.GatewayRepository)),
	wire.Bind(new(configuration.RouteRepository), new(*apiserver.RouteRepository)),
	wire.Bind(new(configuration.UpstreamRepository), new(*apiserver.UpstreamRepository)),
	wire.Bind(new(configuration.CertificateRepository), new(*apiserver.CertificateRepository)),
	wire.Bind(new(configuration.RateLimitPolicyRepository), new(*apiserver.RateLimitPolicyRepository)),
	wire.Bind(new(configuration.AccessControlPolicyRepository), new(*apiserver.AccessControlPolicyRepository)),
	wire.Bind(new(configuration.TokenQuotaPolicyRepository), new(*apiserver.TokenQuotaPolicyRepository)),
)

// Data 持有 Admin API 使用的外部数据连接
type Data struct {
	resourceClient clientset.Interface
	db             *sql.DB
	rdb            *redis.Client
}

// NewData 创建外部数据连接，清理函数由 Kratos App 生命周期统一调用
func NewData(config *conf.Data) (*Data, func(), error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(config.GetApiserver().GetMaster(), config.GetApiserver().GetKubeconfig())
	if err != nil {
		return nil, nil, fmt.Errorf("build apiserver client config: %w", err)
	}
	resourceClient, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("create apiserver client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	db, err := mysqlx.NewDB(ctx, mysqlConfig(config.GetMysql()))
	if err != nil {
		return nil, nil, err
	}
	rdb, err := redisx.NewClient(ctx, redisConfig(config.GetRedis()))
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}

	data := &Data{resourceClient: resourceClient, db: db, rdb: rdb}
	cleanup := func() {
		_ = errors.Join(data.rdb.Close(), data.db.Close())
	}
	return data, cleanup, nil
}

// NewResourceClient 提供声明式资源客户端给各资源 Repository
func NewResourceClient(data *Data) clientset.Interface {
	return data.resourceClient
}

// NewDB 提供 MySQL 连接给 DAO
func NewDB(data *Data) *sql.DB {
	return data.db
}

// NewRedisClient 提供 Redis 连接给执行索引
func NewRedisClient(data *Data) *redis.Client {
	return data.rdb
}

func mysqlConfig(config *conf.Data_MySQL) mysqlx.Config {
	return mysqlx.Config{
		DSN:                   config.GetDsn(),
		MaxOpenConnections:    int(config.GetMaxOpenConnections()),
		MaxIdleConnections:    int(config.GetMaxIdleConnections()),
		ConnectionMaxLifetime: config.GetConnectionMaxLifetime().AsDuration(),
	}
}

func redisConfig(config *conf.Data_Redis) redisx.Config {
	return redisx.Config{
		Address:  config.GetAddress(),
		Password: config.GetPassword(),
		Database: int(config.GetDatabase()),
	}
}
