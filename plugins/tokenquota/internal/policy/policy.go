// Package policy 执行 TokenQuota 插件的预算池匹配和请求裁决
package policy

import config "github.com/lgc202/ingate/pkg/plugin/tokenquota"

// RequestAttributes 保存 Token 配额主体匹配需要读取的请求属性
type RequestAttributes struct {
	RemoteAddr string
	Headers    map[string]string
}

// Check 表示一条需要通过系统 Redis 检查和记账的 Token 配额
type Check struct {
	Policy   config.Policy
	RedisKey string
}

// CheckOutcome 表示 Redis 返回的一次额度检查结果
type CheckOutcome struct {
	Allowed      bool
	Used         int64
	Limit        int64
	ResetSeconds int64
	Err          error
}

// Decision 表示当前请求经过全部 Token 配额后的最终裁决
type Decision struct {
	Allowed    bool
	StatusCode int
	Message    string
	ErrorType  string
	ErrorCode  string
	RetryAfter int64
}
