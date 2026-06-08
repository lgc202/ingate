// Package policy 执行 RateLimit 插件的策略判断
package policy

import (
	"time"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

// Decision 表示一条规则的限流判断结果
type Decision struct {
	Allowed      bool
	StatusCode   int
	Message      string
	QuotaHeaders map[string]string
	Policy       config.Policy
	Rule         config.Rule
	Key          string
}

// Result 表示一次请求经过限流策略后的结果
type Result struct {
	Allowed      bool
	Decision     Decision
	QuotaHeaders map[string]string
	GlobalChecks []GlobalCheck
	Errors       []error
}

// GlobalCheck 表示需要交给 ingate-dataplane 执行的 global limit 检查
type GlobalCheck struct {
	Policy         config.Policy
	Rule           config.Rule
	Key            string
	RedisStore     string
	RedisKey       string
	RedisTimeoutMs int
	Requests       int
	WindowSeconds  int
	Burst          int
}

// Runner 应用限流策略，产出本地决策或 global limit 检查
type Runner struct {
	store counterStore
	now   func() time.Time
}

type window struct {
	start time.Time
	count int
}

// NewMemoryRunner 创建使用内存计数器的策略执行器
func NewMemoryRunner() *Runner {
	return &Runner{
		store: newMemoryCounterStore(),
		now:   time.Now,
	}
}

// NewMemoryRunnerWithClock 创建使用指定时钟的内存策略执行器
func NewMemoryRunnerWithClock(now func() time.Time) *Runner {
	return &Runner{
		store: newMemoryCounterStore(),
		now:   now,
	}
}

// NewSharedDataRunner 使用 Envoy shared data 保存本地限流计数
func NewSharedDataRunner() *Runner {
	return &Runner{
		store: sharedDataCounterStore{},
		now:   time.Now,
	}
}
