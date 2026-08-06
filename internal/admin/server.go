// Package admin 实现 Ingate 管理 API 组件
package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/admin/accesskeyindex"
	"github.com/lgc202/ingate/internal/admin/service"
	"github.com/lgc202/ingate/internal/admin/store"
	"github.com/lgc202/ingate/internal/pkg/httpserver"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	"github.com/lgc202/ingate/pkg/mysqlx"
	"github.com/lgc202/ingate/pkg/redisx"
)

// Server 持有管理 API 运行期间使用的服务与基础设施连接
type Server struct {
	infrastructure *infrastructure
	httpServer     *httpserver.Server
}

type infrastructure struct {
	database    *sql.DB
	redisClient *redis.Client
}

// New 根据进程配置装配管理 API 服务
func New(ctx context.Context, config Config, logger *slog.Logger) (*Server, error) {
	server, _, err := initializeServer(ctx, config, logger)
	return server, err
}

func newInfrastructure(ctx context.Context, config Config) (*infrastructure, func(), error) {
	database, err := mysqlx.NewDB(ctx, config.MySQL)
	if err != nil {
		return nil, nil, err
	}
	redisClient, err := redisx.NewClient(ctx, config.Redis)
	if err != nil {
		return nil, nil, errors.Join(err, database.Close())
	}

	resources := &infrastructure{
		database:    database,
		redisClient: redisClient,
	}
	// Wire 在后续依赖装配失败时调用 cleanup；成功启动后由 Server.Run 关闭连接
	cleanup := func() {
		_ = resources.close()
	}
	return resources, cleanup, nil
}

func newResourceClient(config Config) (clientset.Interface, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(config.APIServer.Master, config.APIServer.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("build apiserver client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create apiserver resource client: %w", err)
	}
	return client, nil
}

func newServices(ctx context.Context, stores *store.Store, accessKeyIndex *accesskeyindex.Index) (*service.Service, error) {
	services := service.New(stores, accessKeyIndex)
	if err := services.AccessKey.Reconcile(ctx); err != nil {
		return nil, fmt.Errorf("reconcile access key index: %w", err)
	}
	return services, nil
}

func newServer(resources *infrastructure, httpServer *httpserver.Server) *Server {
	return &Server{
		infrastructure: resources,
		httpServer:     httpServer,
	}
}

// Run 运行管理 API，并在服务退出后关闭其基础设施连接
func (s *Server) Run(ctx context.Context) error {
	return errors.Join(s.httpServer.Run(ctx), s.infrastructure.close())
}

func (i *infrastructure) close() error {
	return errors.Join(i.redisClient.Close(), i.database.Close())
}
