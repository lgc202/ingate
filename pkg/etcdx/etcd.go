// Package etcdx 定义 Ingate 服务使用的 etcd 存储配置
package etcdx

import (
	"errors"
	"strings"
)

// Config 定义 etcd endpoints 和键空间前缀
type Config struct {
	Endpoints []string `json:"endpoints" mapstructure:"endpoints"`
	Prefix    string   `json:"prefix" mapstructure:"prefix"`
}

// Validate 校验 etcd 存储配置
func (c Config) Validate() error {
	if len(c.Endpoints) == 0 {
		return errors.New("at least one etcd endpoint is required")
	}
	for _, endpoint := range c.Endpoints {
		if strings.TrimSpace(endpoint) == "" {
			return errors.New("etcd endpoint must not be empty")
		}
	}
	if !strings.HasPrefix(c.Prefix, "/") {
		return errors.New("etcd prefix must start with /")
	}
	return nil
}
