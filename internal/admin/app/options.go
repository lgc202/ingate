package app

import (
	"errors"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/pkg/mysqlx"
	"github.com/lgc202/ingate/pkg/redisx"
)

const defaultConfigPath = "configs/ingate-admin.yaml"

// Config 定义 ingate-admin 的进程配置
type Config struct {
	Server    ServerConfig      `json:"server" mapstructure:"server"`
	APIServer APIServerConfig   `json:"apiserver" mapstructure:"apiserver"`
	MySQL     mysqlx.Config     `json:"mysql" mapstructure:"mysql"`
	Redis     redisx.Config     `json:"redis" mapstructure:"redis"`
	Logging   appconfig.Logging `json:"logging" mapstructure:"logging"`
}

// ServerConfig 定义管理 API 的监听配置
type ServerConfig struct {
	ListenAddress string `json:"listen_address" mapstructure:"listen_address"`
}

// APIServerConfig 定义声明式资源 API 连接配置
type APIServerConfig struct {
	Master     string `json:"master" mapstructure:"master"`
	Kubeconfig string `json:"kubeconfig" mapstructure:"kubeconfig"`
}

// Validate 校验进程配置
func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.ListenAddress) == "" {
		return errors.New("server listen address must not be empty")
	}
	if err := c.MySQL.Validate(); err != nil {
		return err
	}
	if err := c.Redis.Validate(); err != nil {
		return err
	}
	return c.Logging.Validate()
}
