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
	apiserver.NewGateway,
	apiserver.NewRoute,
	apiserver.NewUpstream,
	apiserver.NewCertificate,
	apiserver.NewRateLimitPolicy,
	apiserver.NewAccessControlPolicy,
	apiserver.NewTokenQuotaPolicy,
	accesskeydao.New,
	cache.NewCredentialIndex,
	NewAccessKeyRepository,
	wire.Bind(new(biz.GatewayRepository), new(*apiserver.GatewayRepository)),
	wire.Bind(new(biz.RouteRepository), new(*apiserver.RouteRepository)),
	wire.Bind(new(biz.UpstreamRepository), new(*apiserver.UpstreamRepository)),
	wire.Bind(new(biz.CertificateRepository), new(*apiserver.CertificateRepository)),
	wire.Bind(new(biz.RateLimitPolicyRepository), new(*apiserver.RateLimitPolicyRepository)),
	wire.Bind(new(biz.AccessControlPolicyRepository), new(*apiserver.AccessControlPolicyRepository)),
	wire.Bind(new(biz.TokenQuotaPolicyRepository), new(*apiserver.TokenQuotaPolicyRepository)),
	wire.Bind(new(biz.AccessKeyRepository), new(*accessKeyRepository)),
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
