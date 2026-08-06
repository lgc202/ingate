// Package app 实现 ingate-admin 服务入口
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	kitconfig "github.com/lgc202/go-kit/config"
	"github.com/lgc202/go-kit/version"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/admin/accesskeyindex"
	"github.com/lgc202/ingate/internal/admin/handler"
	adminserver "github.com/lgc202/ingate/internal/admin/server"
	"github.com/lgc202/ingate/internal/admin/service"
	"github.com/lgc202/ingate/internal/admin/store"
	"github.com/lgc202/ingate/internal/pkg/httpserver"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

const usage = `ingate-admin 提供 Ingate 管理 API

职责：
  - 聚合声明式资源，提供页面友好的接口
  - 通过 ingate-apiserver 写入网关期望状态
  - 管理客户端访问密钥并发布数据面认证索引
`

// Run 执行 ingate-admin 服务
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-admin", pflag.ContinueOnError)
	flags.SetOutput(stderr)

	configPath := defaultConfigPath
	showVersion := false
	flags.StringVar(&configPath, "config", configPath, "配置文件路径")
	flags.BoolVar(&showVersion, "version", showVersion, "显示版本信息")
	flags.Usage = func() {
		fmt.Fprint(stdout, usage)
		fmt.Fprintln(stdout)
		flags.SetOutput(stdout)
		flags.PrintDefaults()
		flags.SetOutput(stderr)
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}
		return err
	}
	if showVersion {
		fmt.Fprintln(stdout, version.Get().Text())
		return nil
	}

	loaded, err := kitconfig.Load[Config](configPath, kitconfig.WithEnv[Config]("INGATE_ADMIN"))
	if err != nil {
		return err
	}
	settings := loaded.Get()
	if err := settings.Validate(); err != nil {
		return err
	}

	logger, err := settings.Logging.NewLogger("ingate-admin", stdout)
	if err != nil {
		return err
	}
	defer logger.Close()
	loaded.OnChange(func(old, next Config) {
		if err := next.Validate(); err != nil {
			logger.Error("ignoring invalid configuration change", "err", err)
			return
		}
		old.Logging.ApplyDynamic(next.Logging, logger)
		old.Logging = next.Logging
		if kitconfig.Changed(old, next) {
			logger.Warn("service configuration changed and requires a restart")
		}
	})
	logger.Info("service started", "config_file", configPath)

	restConfig, err := clientcmd.BuildConfigFromFlags(settings.APIServer.Master, settings.APIServer.Kubeconfig)
	if err != nil {
		return fmt.Errorf("build apiserver client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("create apiserver resource client: %w", err)
	}
	database, err := sql.Open("mysql", settings.MySQL.DSN)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	database.SetMaxOpenConns(settings.MySQL.MaxOpenConnections)
	database.SetMaxIdleConns(settings.MySQL.MaxIdleConnections)
	database.SetConnMaxLifetime(settings.MySQL.ConnectionMaxLifetime)
	defer database.Close()
	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("connect mysql: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     settings.Redis.Address,
		Password: settings.Redis.Password,
		DB:       settings.Redis.Database,
	})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}

	componentLogger := logger.With("component", "ingate-admin")
	stores := store.New(client, database)
	accessKeyIndex := accesskeyindex.New(redisClient)
	services := service.New(stores, accessKeyIndex)
	if err := services.AccessKey.Reconcile(ctx); err != nil {
		return fmt.Errorf("reconcile access key index: %w", err)
	}
	handlers := handler.New(services, componentLogger)

	gin.SetMode(gin.ReleaseMode)
	router := adminserver.NewRouter(handlers, componentLogger)
	server := httpserver.New(
		settings.Server.ListenAddress,
		router,
		componentLogger,
	)

	return server.Run(ctx)
}
