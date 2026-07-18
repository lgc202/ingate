package app

import (
	"errors"
	"net"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

const defaultConfigPath = "configs/ingate-apiserver.yaml"

// Config 定义 ingate-apiserver 的进程配置
type Config struct {
	Server  ServerConfig      `mapstructure:"server"`
	Storage StorageConfig     `mapstructure:"storage"`
	Logging appconfig.Logging `mapstructure:"logging"`
}

// ServerConfig 定义声明式资源 API 服务配置
type ServerConfig struct {
	BindAddress   string `mapstructure:"bind_address"`
	SecurePort    int    `mapstructure:"secure_port"`
	CertDirectory string `mapstructure:"cert_directory"`
}

// StorageConfig 定义声明式资源持久化配置
type StorageConfig struct {
	EtcdServers []string `mapstructure:"etcd_servers"`
	PathPrefix  string   `mapstructure:"path_prefix"`
}

// Validate 校验进程配置
func (c Config) Validate() error {
	if net.ParseIP(strings.TrimSpace(c.Server.BindAddress)) == nil {
		return errors.New("server bind address must be a valid IP")
	}
	if c.Server.SecurePort < 1 || c.Server.SecurePort > 65535 {
		return errors.New("server secure port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.Server.CertDirectory) == "" {
		return errors.New("server certificate directory must not be empty")
	}
	if len(c.Storage.EtcdServers) == 0 {
		return errors.New("at least one etcd server is required")
	}
	if !strings.HasPrefix(c.Storage.PathPrefix, "/") {
		return errors.New("storage path prefix must start with /")
	}
	return c.Logging.Validate()
}
