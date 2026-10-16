// Package redis 构造原生 Redis 客户端，不负责业务访问或组件生命周期。
package redis

import (
	"crypto/tls"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Config 配置单节点、Sentinel 或 Redis Cluster 客户端。
// 数值和 Duration 的零值沿用驱动默认值。
type Config struct {
	Addrs            []string
	Username         string
	Password         string
	SentinelUsername string
	SentinelPassword string
	DB               int
	MasterName       string
	IsClusterMode    bool
	PoolSize         int
	MinIdleConns     int
	DialTimeout      time.Duration
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	PoolTimeout      time.Duration
	TLS              bool
	TLSServerName    string
}

// New 按原生 UniversalOptions 选择客户端；调用方负责启动探测和关闭。
// 本函数不执行 Ping，驱动仍可能按 MinIdleConns 配置后台建连。
func New(config Config) (goredis.UniversalClient, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validate Redis config: %w", err)
	}

	return goredis.NewUniversalClient(config.options()), nil
}

// Validate 拒绝无效配置及会被原生工厂静默忽略的选项组合。
func (c Config) Validate() error {
	if len(c.Addrs) == 0 {
		return errors.New("addresses are required")
	}

	for i, addr := range c.Addrs {
		if strings.TrimSpace(addr) == "" {
			return fmt.Errorf("address at index %d is empty", i)
		}
	}

	if c.DB < 0 {
		return errors.New("db must be non-negative")
	}

	if c.MasterName == "" && (len(c.Addrs) > 1 || c.IsClusterMode) && c.DB != 0 {
		return errors.New("cluster requires db 0")
	}

	if c.MasterName == "" && (c.SentinelUsername != "" || c.SentinelPassword != "") {
		return errors.New("sentinel credentials require master_name")
	}

	if c.PoolSize < 0 || c.MinIdleConns < 0 {
		return errors.New("pool sizes must be non-negative")
	}

	timeouts := []struct {
		name  string
		value time.Duration
	}{
		{name: "dial_timeout", value: c.DialTimeout},
		{name: "read_timeout", value: c.ReadTimeout},
		{name: "write_timeout", value: c.WriteTimeout},
		{name: "pool_timeout", value: c.PoolTimeout},
	}
	for _, timeout := range timeouts {
		if timeout.value < 0 {
			return fmt.Errorf("%s must be non-negative", timeout.name)
		}
	}

	if !c.TLS && c.TLSServerName != "" {
		return errors.New("tls_server_name requires tls")
	}

	return nil
}

func (c Config) options() *goredis.UniversalOptions {
	options := &goredis.UniversalOptions{
		Addrs:                 slices.Clone(c.Addrs),
		Username:              c.Username,
		Password:              c.Password,
		SentinelUsername:      c.SentinelUsername,
		SentinelPassword:      c.SentinelPassword,
		DB:                    c.DB,
		MasterName:            c.MasterName,
		IsClusterMode:         c.IsClusterMode,
		PoolSize:              c.PoolSize,
		MinIdleConns:          c.MinIdleConns,
		DialTimeout:           c.DialTimeout,
		ReadTimeout:           c.ReadTimeout,
		WriteTimeout:          c.WriteTimeout,
		PoolTimeout:           c.PoolTimeout,
		ContextTimeoutEnabled: true,
	}
	if c.TLS {
		options.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: c.TLSServerName,
		}
	}

	return options
}
