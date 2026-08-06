package app

import (
	"errors"
	"net"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/pkg/etcdx"
)

const defaultConfigPath = "configs/ingate-apiserver.yaml"

// Config 定义 ingate-apiserver 的进程配置
type Config struct {
	Server  ServerConfig      `json:"server" mapstructure:"server"`
	Etcd    etcdx.Config      `json:"etcd" mapstructure:"etcd"`
	Logging appconfig.Logging `json:"logging" mapstructure:"logging"`
}

// ServerConfig 定义声明式资源 API 服务配置
type ServerConfig struct {
	BindAddress   string `json:"bind_address" mapstructure:"bind_address"`
	SecurePort    int    `json:"secure_port" mapstructure:"secure_port"`
	CertDirectory string `json:"cert_directory" mapstructure:"cert_directory"`
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
	if err := c.Etcd.Validate(); err != nil {
		return err
	}
	return c.Logging.Validate()
}
