package config

import (
	"encoding/json"
	"errors"
)

const (
	SchemaVersionV1 = "v1"

	ModeLocal  = "Local"
	ModeGlobal = "Global"

	KeyTypeIP       = "IP"
	KeyTypeHeader   = "Header"
	KeyTypeQuery    = "Query"
	KeyTypeCookie   = "Cookie"
	KeyTypeConsumer = "Consumer"
	KeyTypeRoute    = "Route"
	KeyTypeGateway  = "Gateway"

	AlgorithmFixedWindow = "FixedWindow"

	FailurePolicyFailOpen  = "FailOpen"
	FailurePolicyFailClose = "FailClose"

	defaultRejectedStatusCode = 429
	defaultRejectedMessage    = "Too many requests"
)

// PluginConfig 表示 Listener 级插件配置
type PluginConfig struct {
	SchemaVersion string       `json:"schemaVersion"`
	RedisStores   []RedisStore `json:"redisStores,omitempty"`
}

// RedisStore 表示插件运行时使用的 Redis 连接配置
type RedisStore struct {
	Name                 string `json:"name"`
	DisplayName          string `json:"displayName,omitempty"`
	Mode                 string `json:"mode,omitempty"`
	Address              string `json:"address"`
	DB                   int    `json:"db,omitempty"`
	TLS                  bool   `json:"tls,omitempty"`
	Username             string `json:"username,omitempty"`
	PasswordRef          string `json:"passwordRef,omitempty"`
	ConnectTimeoutMillis int    `json:"connectTimeoutMillis,omitempty"`
	CommandTimeoutMillis int    `json:"commandTimeoutMillis,omitempty"`
}

// RouteConfig 表示 Route 级限流配置
type RouteConfig struct {
	SchemaVersion string    `json:"schemaVersion"`
	GatewayName   string    `json:"gatewayName"`
	RouteName     string    `json:"routeName"`
	RuleName      string    `json:"ruleName,omitempty"`
	Bindings      []Binding `json:"bindings"`
}

// Binding 表示绑定展开后的执行配置
type Binding struct {
	Name     string   `json:"name"`
	Target   Target   `json:"target"`
	Policies []Policy `json:"policies"`
}

// Target 表示绑定目标
type Target struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	RuleName string `json:"ruleName,omitempty"`
}

// Policy 表示限流策略执行配置
type Policy struct {
	Name          string   `json:"name"`
	DisplayName   string   `json:"displayName,omitempty"`
	Mode          string   `json:"mode"`
	Rules         []Rule   `json:"rules"`
	Global        *Global  `json:"global,omitempty"`
	Response      Response `json:"response,omitempty"`
	FailurePolicy string   `json:"failurePolicy,omitempty"`
}

// Rule 表示一条限流规则
type Rule struct {
	Name      string    `json:"name"`
	Key       []KeyPart `json:"key"`
	Limit     Quota     `json:"limit"`
	Algorithm string    `json:"algorithm,omitempty"`
}

// KeyPart 表示限流 key 的一个组成部分
type KeyPart struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// Quota 表示固定窗口限流额度
type Quota struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
}

// Global 表示 Redis-backed global limit 配置
type Global struct {
	RedisRef      string `json:"redisRef"`
	Prefix        string `json:"prefix,omitempty"`
	TimeoutMillis int    `json:"timeoutMillis,omitempty"`
}

// Response 表示超限响应配置
type Response struct {
	StatusCode         int    `json:"statusCode,omitempty"`
	Message            string `json:"message,omitempty"`
	QuotaHeaderEnabled bool   `json:"quotaHeaderEnabled,omitempty"`
}

func ParsePluginConfig(data []byte) (PluginConfig, error) {
	var cfg PluginConfig
	if len(data) == 0 {
		cfg.SchemaVersion = SchemaVersionV1
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return PluginConfig{}, err
	}
	if cfg.SchemaVersion == "" {
		cfg.SchemaVersion = SchemaVersionV1
	}
	if cfg.SchemaVersion != SchemaVersionV1 {
		return PluginConfig{}, errors.New("unsupported rate limit plugin config schema version")
	}
	return cfg, nil
}

func ParseRouteConfig(data []byte) (RouteConfig, error) {
	var cfg RouteConfig
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return RouteConfig{}, err
	}
	if cfg.GatewayName == "" && cfg.RouteName == "" && len(cfg.Bindings) == 0 {
		var envelope struct {
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Value) > 0 {
			if err := json.Unmarshal(envelope.Value, &cfg); err != nil {
				return RouteConfig{}, err
			}
		}
	}
	if cfg.SchemaVersion == "" {
		cfg.SchemaVersion = SchemaVersionV1
	}
	if cfg.SchemaVersion != SchemaVersionV1 {
		return RouteConfig{}, errors.New("unsupported rate limit route config schema version")
	}
	return cfg, nil
}

func (p Policy) RejectedStatusCode() int {
	if p.Response.StatusCode > 0 {
		return p.Response.StatusCode
	}
	return defaultRejectedStatusCode
}

func (p Policy) RejectedMessage() string {
	if p.Response.Message != "" {
		return p.Response.Message
	}
	return defaultRejectedMessage
}

func (p Policy) FailOpen() bool {
	return p.FailurePolicy == "" || p.FailurePolicy == FailurePolicyFailOpen
}
