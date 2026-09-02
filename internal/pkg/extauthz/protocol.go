// Package extauthz 定义 Controller、Authz 和 ALS 共享的 Envoy External Authorization 内部协议。
package extauthz

import (
	"encoding/json"
	"fmt"
)

const (
	// FilterName 是 Envoy External Authorization 过滤器的标准名称。
	FilterName = "envoy.filters.http.ext_authz"
	// RouteIDContext 把当前 Ingate Route ID 传给鉴权服务。
	RouteIDContext = "route_id"
	// CallerRequiredContext 表示当前 Route 必须完成 Caller 身份与权限校验。
	CallerRequiredContext = "caller_required"
	// RateLimitsContext 保存 Controller 为当前 Route 编译出的限流规则。
	RateLimitsContext = "rate_limits"
	// MetadataNamespace 是鉴权结果写入 Envoy dynamic metadata 的命名空间。
	MetadataNamespace = FilterName
	// CallerIDField 记录本次请求归属的 Caller ID。
	CallerIDField = "caller_id"
	// AccessKeyIDField 记录本次请求使用的访问密钥 ID。
	AccessKeyIDField = "access_key_id"
)

const (
	// 每条规则会触发一次共享计数操作；同时限制数量和编码大小，避免内部协议放大请求成本。
	maxRateLimitRules       = 64
	maxRateLimitContextSize = 64 << 10
)

// RateLimitSubject 表示请求限流计数器的划分方式。
type RateLimitSubject string

const (
	// RateLimitSubjectShared 表示命中同一策略作用域的请求共享计数器。
	RateLimitSubjectShared RateLimitSubject = "Shared"
	// RateLimitSubjectIP 表示每个客户端 IP 使用独立计数器。
	RateLimitSubjectIP RateLimitSubject = "IP"
	// RateLimitSubjectHeader 表示每个指定 Header 值使用独立计数器。
	RateLimitSubjectHeader RateLimitSubject = "Header"
)

// RateLimitRule 是 Controller 传给 Authz 的单条可执行限流规则。
// 该结构只属于控制面与执行组件之间的内部协议，不暴露为用户可编辑配置。
type RateLimitRule struct {
	PolicyID      string           `json:"policy_id"`
	Scope         string           `json:"scope"`
	Subject       RateLimitSubject `json:"subject"`
	HeaderName    string           `json:"header_name,omitempty"`
	Requests      int64            `json:"requests"`
	WindowSeconds int64            `json:"window_seconds"`
}

// EncodeRateLimitRules 把当前 Route 的规则编码进 Envoy context_extensions。
func EncodeRateLimitRules(rules []RateLimitRule) (string, error) {
	if len(rules) > maxRateLimitRules {
		return "", fmt.Errorf(
			"encode rate limit rules: rule count %d exceeds maximum %d",
			len(rules),
			maxRateLimitRules,
		)
	}
	encoded, err := json.Marshal(rules)
	if err != nil {
		return "", fmt.Errorf("encode rate limit rules: %w", err)
	}
	if len(encoded) > maxRateLimitContextSize {
		return "", fmt.Errorf(
			"encode rate limit rules: encoded size %d exceeds maximum %d bytes",
			len(encoded),
			maxRateLimitContextSize,
		)
	}
	return string(encoded), nil
}

// DecodeRateLimitRules 解码 Envoy 随鉴权请求传入的限流规则。
func DecodeRateLimitRules(value string) ([]RateLimitRule, error) {
	if value == "" {
		return nil, nil
	}
	if len(value) > maxRateLimitContextSize {
		return nil, fmt.Errorf(
			"decode rate limit rules: encoded size %d exceeds maximum %d bytes",
			len(value),
			maxRateLimitContextSize,
		)
	}
	var rules []RateLimitRule
	if err := json.Unmarshal([]byte(value), &rules); err != nil {
		return nil, fmt.Errorf("decode rate limit rules: %w", err)
	}
	if len(rules) > maxRateLimitRules {
		return nil, fmt.Errorf(
			"decode rate limit rules: rule count %d exceeds maximum %d",
			len(rules),
			maxRateLimitRules,
		)
	}
	return rules, nil
}
