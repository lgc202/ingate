package ratelimit

import (
	"encoding/json"
	"errors"
)

const schemaVersion = "v1"

// Mode 表示限流策略执行模式
type Mode string

const (
	ModeLocal  Mode = "Local"
	ModeGlobal Mode = "Global"
)

// KeyType 表示限流 key 的组成维度
type KeyType string

const (
	KeyTypeIP        KeyType = "IP"
	KeyTypeHeader    KeyType = "Header"
	KeyTypeQuery     KeyType = "Query"
	KeyTypeCookie    KeyType = "Cookie"
	KeyTypeConsumer  KeyType = "Consumer"
	KeyTypeRoute     KeyType = "Route"
	KeyTypeGateway   KeyType = "Gateway"
	KeyTypeRouteRule KeyType = "RouteRule"
	KeyTypeJWTClaim  KeyType = "JWTClaim"
	KeyTypeAPIKey    KeyType = "APIKey"
	KeyTypeTenant    KeyType = "Tenant"
)

// Algorithm 表示限流算法
type Algorithm string

const (
	AlgorithmFixedWindow   Algorithm = "FixedWindow"
	AlgorithmSlidingWindow Algorithm = "SlidingWindow"
	AlgorithmTokenBucket   Algorithm = "TokenBucket"
)

// FailurePolicy 表示限流执行异常时的处理方式
type FailurePolicy string

const (
	FailurePolicyFailOpen  FailurePolicy = "FailOpen"
	FailurePolicyFailClose FailurePolicy = "FailClose"
)

const (
	defaultRejectedStatusCode = 429
	defaultRejectedMessage    = "Too many requests"
)

// Config 表示 xDS target 中保留的 RateLimit 插件编译结果
type Config struct {
	SchemaVersion string       `json:"schemaVersion"`
	Bindings      []Binding    `json:"bindings,omitempty"`
	RedisStores   []RedisStore `json:"redisStores,omitempty"`
	DataPlane     *DataPlane   `json:"dataPlane,omitempty"`
}

// PluginConfig 表示真正下发给 Wasm 插件的运行时配置
type PluginConfig struct {
	SchemaVersion string        `json:"schemaVersion"`
	RedisStores   []RedisStore  `json:"redisStores,omitempty"`
	DataPlane     *DataPlane    `json:"dataPlane,omitempty"`
	Routes        []RouteConfig `json:"routes,omitempty"`
}

// RedisStore 表示插件运行时使用的 Redis 连接配置
type RedisStore struct {
	Name                 string   `json:"name"`
	DisplayName          string   `json:"displayName,omitempty"`
	Mode                 string   `json:"mode,omitempty"`
	Address              string   `json:"address"`
	Addresses            []string `json:"addresses,omitempty"`
	DB                   int      `json:"db,omitempty"`
	TLS                  bool     `json:"tls,omitempty"`
	TLSServerName        string   `json:"tlsServerName,omitempty"`
	Username             string   `json:"username,omitempty"`
	PasswordRef          string   `json:"passwordRef,omitempty"`
	ConnectTimeoutMillis int      `json:"connectTimeoutMillis,omitempty"`
	CommandTimeoutMillis int      `json:"commandTimeoutMillis,omitempty"`
	PoolSize             int      `json:"poolSize,omitempty"`
	MinIdleConns         int      `json:"minIdleConns,omitempty"`
	SentinelMaster       string   `json:"sentinelMaster,omitempty"`
}

// DataPlane 表示插件调用 ingate-dataplane 所需的 Envoy cluster 配置
type DataPlane struct {
	ClusterName   string `json:"clusterName"`
	Path          string `json:"path"`
	TimeoutMillis int    `json:"timeoutMillis"`
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
	Name          string        `json:"name"`
	DisplayName   string        `json:"displayName,omitempty"`
	Mode          Mode          `json:"mode"`
	Rules         []Rule        `json:"rules"`
	Global        *Global       `json:"global,omitempty"`
	Response      Response      `json:"response,omitempty"`
	FailurePolicy FailurePolicy `json:"failurePolicy,omitempty"`
}

// Rule 表示一条限流规则
type Rule struct {
	Name      string    `json:"name"`
	Key       []KeyPart `json:"key"`
	Limit     Quota     `json:"limit"`
	Algorithm Algorithm `json:"algorithm,omitempty"`
}

// KeyPart 表示限流 key 的一个组成部分
type KeyPart struct {
	Type KeyType `json:"type"`
	Name string  `json:"name,omitempty"`
}

// Quota 表示固定窗口限流额度
type Quota struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
	Burst         int `json:"burst,omitempty"`
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

// ParsePluginConfig 解析 Listener 级插件配置
func ParsePluginConfig(data []byte) (PluginConfig, error) {
	var cfg PluginConfig
	if len(data) == 0 {
		cfg.SchemaVersion = schemaVersion
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return PluginConfig{}, err
	}
	if cfg.SchemaVersion == "" {
		cfg.SchemaVersion = schemaVersion
	}
	if cfg.SchemaVersion != schemaVersion {
		return PluginConfig{}, errors.New("unsupported rate limit plugin config schema version")
	}
	return cfg, nil
}

// ParseRouteConfig 解析 route 级插件配置
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
		cfg.SchemaVersion = schemaVersion
	}
	if cfg.SchemaVersion != schemaVersion {
		return RouteConfig{}, errors.New("unsupported rate limit route config schema version")
	}
	return cfg, nil
}

// RejectedStatusCode 返回超限响应状态码
func (p Policy) RejectedStatusCode() int {
	if p.Response.StatusCode > 0 {
		return p.Response.StatusCode
	}
	return defaultRejectedStatusCode
}

// RejectedMessage 返回超限响应消息
func (p Policy) RejectedMessage() string {
	if p.Response.Message != "" {
		return p.Response.Message
	}
	return defaultRejectedMessage
}

// FailOpen 表示限流执行失败时是否放行请求
func (p Policy) FailOpen() bool {
	return p.FailurePolicy == "" || p.FailurePolicy == FailurePolicyFailOpen
}
