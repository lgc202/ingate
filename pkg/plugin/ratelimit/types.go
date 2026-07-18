package ratelimit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// KeyType 表示限流 key 的组成维度
type KeyType string

const (
	KeyTypeIP        KeyType = "IP"
	KeyTypeHeader    KeyType = "Header"
	KeyTypeQuery     KeyType = "Query"
	KeyTypeCookie    KeyType = "Cookie"
	KeyTypeRoute     KeyType = "Route"
	KeyTypeGateway   KeyType = "Gateway"
	KeyTypeRouteRule KeyType = "RouteRule"
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

// PluginConfig 表示真正下发给 Wasm 插件的运行时配置
type PluginConfig struct {
	Routes []RouteConfig `json:"routes"`
}

// RouteConfig 表示 Route 级限流配置
type RouteConfig struct {
	GatewayName string   `json:"gatewayName"`
	RouteName   string   `json:"routeName"`
	Policies    []Policy `json:"policies"`
}

// Policy 表示限流策略执行配置
type Policy struct {
	Name          string        `json:"name"`
	Scope         string        `json:"scope"`
	Rules         []Rule        `json:"rules"`
	Response      Response      `json:"response,omitempty"`
	FailurePolicy FailurePolicy `json:"failurePolicy,omitempty"`
}

// Rule 表示一条限流规则
type Rule struct {
	Name  string    `json:"name"`
	Key   []KeyPart `json:"key"`
	Limit Quota     `json:"limit"`
}

// KeyPart 表示限流 key 的一个组成部分
type KeyPart struct {
	Type KeyType `json:"type"`
	Name string  `json:"name,omitempty"`
}

// Quota 表示限流额度
type Quota struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
	// Burst 表示令牌桶容量，省略时使用 Requests
	Burst int `json:"burst,omitempty"`
}

// Response 表示超限响应配置
type Response struct {
	StatusCode         int    `json:"statusCode,omitempty"`
	Message            string `json:"message,omitempty"`
	QuotaHeaderEnabled bool   `json:"quotaHeaderEnabled,omitempty"`
}

// ParsePluginConfig 严格解析 Listener 级插件配置
func ParsePluginConfig(data []byte) (PluginConfig, error) {
	var cfg PluginConfig
	if err := decodeStrict(data, &cfg); err != nil {
		return PluginConfig{}, err
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

func decodeStrict(data []byte, value any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("rate limit config must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("rate limit config contains multiple JSON values")
		}
		return err
	}
	return nil
}
