// Package policy 执行 RateLimit 插件的策略判断
package policy

import (
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

// Decision 表示一条规则的限流判断结果
type Decision struct {
	Allowed      bool
	StatusCode   int
	Message      string
	QuotaHeaders map[string]string
}

// Check 表示需要通过系统 Redis 执行的一条限流检查
type Check struct {
	Policy   config.Policy
	Rule     config.Rule
	RedisKey string
}

// Outcome 表示一条 Redis 限流检查的执行结果
type Outcome struct {
	Allowed      bool
	Current      int
	Limit        int
	ResetSeconds int
	Err          error
}
