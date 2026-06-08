package acl

import (
	"encoding/json"
	"errors"
)

const schemaVersion = "v1"

// Action 表示 ACL 规则命中后的处理动作
type Action string

const (
	ActionAllow Action = "Allow"
	ActionDeny  Action = "Deny"
)

// ConditionType 表示 ACL 规则的匹配维度
type ConditionType string

const (
	ConditionTypeIP       ConditionType = "IP"
	ConditionTypeHeader   ConditionType = "Header"
	ConditionTypeConsumer ConditionType = "Consumer"
	ConditionTypeTenant   ConditionType = "Tenant"
)

const (
	defaultDeniedStatusCode = 403
	defaultDeniedMessage    = "Access denied"
)

// PluginConfig 表示真正下发给 Wasm 插件的 ACL 运行时配置
type PluginConfig struct {
	SchemaVersion string        `json:"schemaVersion"`
	Routes        []RouteConfig `json:"routes,omitempty"`
}

// RouteConfig 表示 Route 级 ACL 配置
type RouteConfig struct {
	SchemaVersion string   `json:"schemaVersion"`
	GatewayName   string   `json:"gatewayName"`
	RouteName     string   `json:"routeName"`
	RuleName      string   `json:"ruleName,omitempty"`
	DefaultAction Action   `json:"defaultAction,omitempty"`
	Rules         []Rule   `json:"rules,omitempty"`
	Response      Response `json:"response,omitempty"`
}

// Rule 表示一条 ACL 规则，Conditions 全部命中后才执行 Action
type Rule struct {
	Name       string      `json:"name"`
	Action     Action      `json:"action"`
	Conditions []Condition `json:"conditions,omitempty"`
}

// Condition 表示 ACL 规则中的一个匹配条件
type Condition struct {
	Type  ConditionType `json:"type"`
	Name  string        `json:"name,omitempty"`
	Value string        `json:"value"`
}

// Response 表示 ACL 拒绝请求时的响应配置
type Response struct {
	StatusCode int    `json:"statusCode,omitempty"`
	Message    string `json:"message,omitempty"`
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
		return PluginConfig{}, errors.New("unsupported acl plugin config schema version")
	}
	return cfg, nil
}

// DeniedStatusCode 返回 ACL 拒绝请求时的响应状态码
func (r RouteConfig) DeniedStatusCode() int {
	if r.Response.StatusCode > 0 {
		return r.Response.StatusCode
	}
	return defaultDeniedStatusCode
}

// DeniedMessage 返回 ACL 拒绝请求时的响应消息
func (r RouteConfig) DeniedMessage() string {
	if r.Response.Message != "" {
		return r.Response.Message
	}
	return defaultDeniedMessage
}
