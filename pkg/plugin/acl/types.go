package acl

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// Action 表示 ACL 规则命中后的处理动作
type Action string

const (
	ActionAllow Action = "Allow"
	ActionDeny  Action = "Deny"
)

// ConditionType 表示 ACL 规则的匹配维度
type ConditionType string

const (
	ConditionTypeIP     ConditionType = "IP"
	ConditionTypeHeader ConditionType = "Header"
)

const (
	defaultDeniedStatusCode = 403
	defaultDeniedMessage    = "Access denied"
)

// PluginConfig 表示真正下发给 ACL Wasm 插件的 Listener 级执行配置
type PluginConfig struct {
	Routes []RouteConfig `json:"routes"`
}

// RouteConfig 表示 Route 级 ACL 配置
type RouteConfig struct {
	GatewayName string   `json:"gatewayName"`
	RouteName   string   `json:"routeName"`
	Policies    []Policy `json:"policies"`
}

// Policy 表示访问控制策略执行配置
type Policy struct {
	Name          string   `json:"name"`
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

// ParsePluginConfig 严格解析 Listener 级插件配置
func ParsePluginConfig(data []byte) (PluginConfig, error) {
	var cfg PluginConfig
	if err := decodeStrict(data, &cfg); err != nil {
		return PluginConfig{}, err
	}
	return cfg, nil
}

// DeniedStatusCode 返回 ACL 拒绝请求时的响应状态码
func (p Policy) DeniedStatusCode() int {
	if p.Response.StatusCode > 0 {
		return p.Response.StatusCode
	}
	return defaultDeniedStatusCode
}

// DeniedMessage 返回访问控制拒绝请求时的响应消息
func (p Policy) DeniedMessage() string {
	if p.Response.Message != "" {
		return p.Response.Message
	}
	return defaultDeniedMessage
}

func decodeStrict(data []byte, value any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("access control config must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("access control config contains multiple JSON values")
		}
		return err
	}
	return nil
}
