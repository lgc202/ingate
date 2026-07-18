package app

import (
	"errors"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

const defaultConfigPath = "configs/ingate-admin-api.yaml"

// Config 定义 ingate-admin-api 的进程配置
type Config struct {
	Server    ServerConfig      `mapstructure:"server"`
	APIServer APIServerConfig   `mapstructure:"apiserver"`
	Logging   appconfig.Logging `mapstructure:"logging"`
}

// ServerConfig 定义管理 API 服务配置
type ServerConfig struct {
	ListenAddress string `mapstructure:"listen_address"`
	ConsoleDir    string `mapstructure:"console_dir"`
}

// APIServerConfig 定义声明式资源 API 连接配置
type APIServerConfig struct {
	Master     string `mapstructure:"master"`
	Kubeconfig string `mapstructure:"kubeconfig"`
}

// Validate 校验进程配置
func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.ListenAddress) == "" {
		return errors.New("server listen address must not be empty")
	}
	return c.Logging.Validate()
}
