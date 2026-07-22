package tokenquota

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	defaultRejectedStatusCode = 429
	defaultRejectedMessage    = "Token quota exceeded"
	maxSafeInteger            = int64(1<<53 - 1)
	maxWindowSeconds          = int64(1<<31 - 1)
)

// SubjectType 表示额度预算池的请求主体维度
type SubjectType string

const (
	// SubjectTypeShared 表示所有请求共享同一个 Policy 预算池
	SubjectTypeShared SubjectType = "Shared"
	// SubjectTypeIP 表示按网关看到的来源 IP 划分 Policy 预算池
	SubjectTypeIP SubjectType = "IP"
	// SubjectTypeHeader 表示按指定请求 Header 划分 Policy 预算池
	SubjectTypeHeader SubjectType = "Header"
)

// FailurePolicy 表示请求前额度检查异常时的处理方式
type FailurePolicy string

const (
	// FailurePolicyFailOpen 表示额度检查异常时放行请求
	FailurePolicyFailOpen FailurePolicy = "FailOpen"
	// FailurePolicyFailClose 表示额度检查异常时拒绝请求
	FailurePolicyFailClose FailurePolicy = "FailClose"
)

// PluginConfig 表示真正下发给 TokenQuota Wasm 插件的 Listener 级执行配置
type PluginConfig struct {
	Routes []RouteConfig `json:"routes"`
}

// RouteConfig 表示精确到 RouteRule 的 Token 配额执行配置
type RouteConfig struct {
	GatewayName string   `json:"gatewayName"`
	RouteName   string   `json:"routeName"`
	RuleName    string   `json:"ruleName"`
	Policies    []Policy `json:"policies"`
}

// Policy 表示一个跨全部 targetRefs 共享的 Token 预算池
type Policy struct {
	Name          string        `json:"name"`
	BudgetID      string        `json:"budgetID"`
	Subject       Subject       `json:"subject"`
	Quota         Quota         `json:"quota"`
	FailurePolicy FailurePolicy `json:"failurePolicy"`
	Response      Response      `json:"response,omitempty"`
}

// Subject 定义额度预算池的请求主体
type Subject struct {
	Type       SubjectType `json:"type"`
	HeaderName string      `json:"headerName,omitempty"`
}

// Quota 定义固定窗口内允许使用的 Token 数
type Quota struct {
	Tokens        int64 `json:"tokens"`
	WindowSeconds int64 `json:"windowSeconds"`
}

// Response 定义额度耗尽时返回给调用方的响应
type Response struct {
	Message string `json:"message,omitempty"`
}

// ParsePluginConfig 严格解析 Listener 级 TokenQuota 插件配置
func ParsePluginConfig(data []byte) (PluginConfig, error) {
	var cfg PluginConfig
	if err := decodeStrict(data, &cfg); err != nil {
		return PluginConfig{}, err
	}
	if err := cfg.validate(); err != nil {
		return PluginConfig{}, err
	}
	return cfg, nil
}

// RejectedStatusCode 返回额度耗尽时的 HTTP 状态码
func (p Policy) RejectedStatusCode() int {
	return defaultRejectedStatusCode
}

// RejectedMessage 返回额度耗尽时的稳定错误文案
func (p Policy) RejectedMessage() string {
	if p.Response.Message != "" {
		return p.Response.Message
	}
	return defaultRejectedMessage
}

// FailOpen 返回请求前额度检查异常时是否继续放行
func (p Policy) FailOpen() bool {
	return p.FailurePolicy == FailurePolicyFailOpen
}

func (c *PluginConfig) validate() error {
	routes := make(map[string]bool, len(c.Routes))
	for i := range c.Routes {
		route := &c.Routes[i]
		if route.GatewayName == "" {
			return fmt.Errorf("routes[%d].gatewayName must not be empty", i)
		}
		if route.RouteName == "" {
			return fmt.Errorf("routes[%d].routeName must not be empty", i)
		}
		if route.RuleName == "" {
			return fmt.Errorf("routes[%d].ruleName must not be empty", i)
		}
		if len(route.Policies) == 0 {
			return fmt.Errorf("routes[%d].policies must not be empty", i)
		}

		routeKey := route.GatewayName + "\x00" + route.RouteName + "\x00" + route.RuleName
		if routes[routeKey] {
			return fmt.Errorf("routes[%d] duplicates route rule %q/%q/%q", i, route.GatewayName, route.RouteName, route.RuleName)
		}
		routes[routeKey] = true

		policies := make(map[string]bool, len(route.Policies))
		for j := range route.Policies {
			policy := &route.Policies[j]
			if err := policy.validate(i, j); err != nil {
				return err
			}
			if policies[policy.Name] {
				return fmt.Errorf("routes[%d].policies[%d] duplicates policy %q", i, j, policy.Name)
			}
			policies[policy.Name] = true
		}
	}
	return nil
}

func (p *Policy) validate(routeIndex, policyIndex int) error {
	prefix := fmt.Sprintf("routes[%d].policies[%d]", routeIndex, policyIndex)
	if p.Name == "" {
		return fmt.Errorf("%s.name must not be empty", prefix)
	}
	if p.BudgetID == "" {
		return fmt.Errorf("%s.budgetID must not be empty", prefix)
	}
	p.Subject.HeaderName = strings.ToLower(strings.TrimSpace(p.Subject.HeaderName))
	switch p.Subject.Type {
	case SubjectTypeShared, SubjectTypeIP:
		if p.Subject.HeaderName != "" {
			return fmt.Errorf("%s.subject.headerName must be empty for subject type %q", prefix, p.Subject.Type)
		}
	case SubjectTypeHeader:
		if !validHeaderName(p.Subject.HeaderName) {
			return fmt.Errorf("%s.subject.headerName is invalid", prefix)
		}
	default:
		return fmt.Errorf("%s.subject.type %q is not supported", prefix, p.Subject.Type)
	}
	if p.Quota.Tokens <= 0 || p.Quota.Tokens > maxSafeInteger {
		return fmt.Errorf("%s.quota.tokens must be between 1 and %d", prefix, maxSafeInteger)
	}
	if p.Quota.WindowSeconds <= 0 || p.Quota.WindowSeconds > maxWindowSeconds {
		return fmt.Errorf("%s.quota.windowSeconds must be between 1 and %d", prefix, maxWindowSeconds)
	}
	switch p.FailurePolicy {
	case FailurePolicyFailOpen, FailurePolicyFailClose:
	default:
		return fmt.Errorf("%s.failurePolicy %q is not supported", prefix, p.FailurePolicy)
	}
	return nil
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, value := range []byte(name) {
		if !headerTokenByte(value) {
			return false
		}
	}
	return true
}

func headerTokenByte(value byte) bool {
	if value >= '0' && value <= '9' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' {
		return true
	}
	switch value {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	default:
		return false
	}
}

func decodeStrict(data []byte, value any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("token quota config must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("token quota config contains multiple JSON values")
		}
		return err
	}
	return nil
}
