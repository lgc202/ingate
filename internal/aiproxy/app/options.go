package app

import (
	"fmt"
	"net"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/pkg/redisx"
)

const defaultConfigPath = "configs/ingate-ai-proxy.yaml"

// Config 定义 ingate-ai-proxy 的进程配置
type Config struct {
	Server  ServerConfig      `json:"server" mapstructure:"server"`
	Redis   redisx.Config     `json:"redis" mapstructure:"redis"`
	Logging appconfig.Logging `json:"logging" mapstructure:"logging"`
}

// ServerConfig 定义进程提供的 HTTP 和 gRPC 服务
type ServerConfig struct {
	HTTP ListenConfig `json:"http" mapstructure:"http"`
	GRPC ListenConfig `json:"grpc" mapstructure:"grpc"`
}

// ListenConfig 定义单个服务监听的 IP 地址和端口
type ListenConfig struct {
	Address string `json:"address" mapstructure:"address"`
	Port    int    `json:"port" mapstructure:"port"`
}

// Validate 校验进程配置
func (c Config) Validate() error {
	if err := c.Server.HTTP.validate("http"); err != nil {
		return err
	}
	if err := c.Server.GRPC.validate("gRPC"); err != nil {
		return err
	}
	if err := c.Redis.Validate(); err != nil {
		return err
	}
	return c.Logging.Validate()
}

func (c ListenConfig) validate(protocol string) error {
	if net.ParseIP(c.Address) == nil {
		return fmt.Errorf("%s server address must be a valid IP", protocol)
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("%s server port must be between 1 and 65535", protocol)
	}
	return nil
}
