package app

import (
	"errors"
	"net/netip"
	"strings"
	"time"

	"github.com/lgc202/ingate/internal/controller/delivery"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

const defaultConfigPath = "configs/ingate-controller.yaml"

// Config 定义 ingate-controller 的进程配置
type Config struct {
	APIServer     APIServerConfig     `mapstructure:"apiserver"`
	Server        ServerConfig        `mapstructure:"server"`
	Delivery      DeliveryConfig      `mapstructure:"delivery"`
	ResourceWatch ResourceWatchConfig `mapstructure:"resource_watch"`
	Logging       appconfig.Logging   `mapstructure:"logging"`
}

// APIServerConfig 定义声明式资源 API 连接配置
type APIServerConfig struct {
	Master     string `mapstructure:"master"`
	Kubeconfig string `mapstructure:"kubeconfig"`
}

// ServerConfig 定义 Controller 对内服务地址
type ServerConfig struct {
	XDSListenAddress    string `mapstructure:"xds_listen_address"`
	HealthListenAddress string `mapstructure:"health_listen_address"`
}

// DeliveryConfig 定义配置发布时序参数
type DeliveryConfig struct {
	CandidateACKTimeout time.Duration `mapstructure:"candidate_ack_timeout"`
	NACKRollbackTimeout time.Duration `mapstructure:"nack_rollback_timeout"`
}

// ResourceWatchConfig 定义声明式资源监听参数
type ResourceWatchConfig struct {
	ResyncPeriod time.Duration `mapstructure:"resync_period"`
}

// Validate 校验进程配置
func (c Config) Validate() error {
	xdsAddress, err := netip.ParseAddrPort(strings.TrimSpace(c.Server.XDSListenAddress))
	if err != nil || !xdsAddress.Addr().Unmap().IsLoopback() {
		return errors.New("xDS listen address must use a loopback IP:port because xDS carries sensitive configuration; remote deployments require mTLS")
	}
	if strings.TrimSpace(c.Server.HealthListenAddress) == "" {
		return errors.New("health listen address must not be empty")
	}
	if c.Delivery.CandidateACKTimeout < 0 {
		return errors.New("candidate ACK timeout must not be negative")
	}
	if c.Delivery.NACKRollbackTimeout < 0 {
		return errors.New("NACK rollback timeout must not be negative")
	}
	if c.ResourceWatch.ResyncPeriod < 0 {
		return errors.New("resource resync period must not be negative")
	}
	return c.Logging.Validate()
}

// DefaultConfig 返回配置文件未覆盖时使用的时序参数
func DefaultConfig() Config {
	return Config{
		Delivery: DeliveryConfig{
			CandidateACKTimeout: delivery.DefaultACKTimeout,
			NACKRollbackTimeout: delivery.DefaultNACKRollbackTimeout,
		},
	}
}
