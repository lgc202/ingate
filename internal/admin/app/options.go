package app

import (
	"errors"
	"strings"
	"time"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

const defaultConfigPath = "configs/ingate-admin.yaml"

// Config 定义 ingate-admin 的进程配置
type Config struct {
	Server    ServerConfig      `mapstructure:"server"`
	APIServer APIServerConfig   `mapstructure:"apiserver"`
	MySQL     MySQLConfig       `mapstructure:"mysql"`
	Redis     RedisConfig       `mapstructure:"redis"`
	Logging   appconfig.Logging `mapstructure:"logging"`
}

// ServerConfig 定义管理 API 的监听配置
type ServerConfig struct {
	ListenAddress string `mapstructure:"listen_address"`
}

// APIServerConfig 定义声明式资源 API 连接配置
type APIServerConfig struct {
	Master     string `mapstructure:"master"`
	Kubeconfig string `mapstructure:"kubeconfig"`
}

// MySQLConfig 定义管理业务数据的 MySQL 连接池
type MySQLConfig struct {
	DSN                   string        `mapstructure:"dsn"`
	MaxOpenConnections    int           `mapstructure:"max_open_connections"`
	MaxIdleConnections    int           `mapstructure:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `mapstructure:"connection_max_lifetime"`
}

// RedisConfig 定义访问密钥等数据面索引使用的系统 Redis
type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	Database int    `mapstructure:"database"`
}

// Validate 校验进程配置
func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.ListenAddress) == "" {
		return errors.New("server listen address must not be empty")
	}
	if strings.TrimSpace(c.MySQL.DSN) == "" {
		return errors.New("mysql DSN must not be empty")
	}
	if c.MySQL.MaxOpenConnections <= 0 {
		return errors.New("mysql max open connections must be greater than zero")
	}
	if c.MySQL.MaxIdleConnections < 0 || c.MySQL.MaxIdleConnections > c.MySQL.MaxOpenConnections {
		return errors.New("mysql max idle connections must be between zero and max open connections")
	}
	if c.MySQL.ConnectionMaxLifetime <= 0 {
		return errors.New("mysql connection max lifetime must be greater than zero")
	}
	if strings.TrimSpace(c.Redis.Address) == "" {
		return errors.New("redis address must not be empty")
	}
	if c.Redis.Database < 0 {
		return errors.New("redis database must not be negative")
	}
	return c.Logging.Validate()
}
