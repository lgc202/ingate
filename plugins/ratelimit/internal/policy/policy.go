// Package policy 执行 RateLimit 插件的策略判断
package policy

import (
	"errors"
	"fmt"
	"time"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

const (
	quotaHeaderLimit     = "X-RateLimit-Limit"
	quotaHeaderRemaining = "X-RateLimit-Remaining"
	quotaHeaderReset     = "X-RateLimit-Reset"
)

var errLocalCounterUnavailable = errors.New("local rate limit counter unavailable")

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

// Apply 对一次请求应用路由限流策略
func (r *Runner) Apply(route config.RouteConfig, req Request) Result {
	result := Result{Allowed: true}
	for _, binding := range route.Bindings {
		for _, rateLimitPolicy := range binding.Policies {
			for _, rule := range rateLimitPolicy.Rules {
				decision, globalCheck, err := r.applyRule(route, binding, rateLimitPolicy, rule, req)
				if err != nil {
					result.Errors = append(result.Errors, err)
				}
				if !decision.Allowed {
					result.Allowed = false
					result.Decision = decision
					return result
				}
				if globalCheck != nil {
					result.GlobalChecks = append(result.GlobalChecks, *globalCheck)
				}
				if len(decision.QuotaHeaders) > 0 {
					result.QuotaHeaders = decision.QuotaHeaders
				}
			}
		}
	}
	return result
}

func (r *Runner) applyRule(route config.RouteConfig, binding config.Binding, rateLimitPolicy config.Policy, rule config.Rule, req Request) (Decision, *GlobalCheck, error) {
	key, ok := compositeKey(req, rule.Key)
	if !ok {
		return Decision{Allowed: true}, nil, nil
	}

	requests := rule.Limit.Requests
	windowSeconds := rule.Limit.WindowSeconds
	if requests <= 0 || windowSeconds <= 0 {
		return Decision{Allowed: true}, nil, nil
	}

	limitKey := limitKey(route, binding, rateLimitPolicy, rule, key)
	if rateLimitPolicy.Mode == config.ModeGlobal {
		redisRef := ""
		timeoutMs := 0
		prefix := "ingate-rate-limit"
		if rateLimitPolicy.Global != nil {
			redisRef = rateLimitPolicy.Global.RedisRef
			timeoutMs = rateLimitPolicy.Global.TimeoutMillis
			if rateLimitPolicy.Global.Prefix != "" {
				prefix = rateLimitPolicy.Global.Prefix
			}
		}
		return Decision{Allowed: true}, &GlobalCheck{
			Policy:         rateLimitPolicy,
			Rule:           rule,
			Key:            key,
			RedisStore:     redisRef,
			RedisKey:       prefix + ":" + limitKey,
			RedisTimeoutMs: timeoutMs,
			Requests:       requests,
			WindowSeconds:  windowSeconds,
			Burst:          rule.Limit.Burst,
		}, nil
	}

	now := r.now()
	current, err := r.store.Increment(limitKey, now, windowSeconds)
	if err != nil {
		storeErr := fmt.Errorf("%w: %s", errLocalCounterUnavailable, err)
		if rateLimitPolicy.FailOpen() {
			return Decision{Allowed: true}, nil, storeErr
		}
		return rejectDecision(rateLimitPolicy, rule, key, requests, 0, windowSeconds), nil, storeErr
	}

	remaining := max(requests-current.count, 0)
	if current.count > requests {
		return rejectDecision(rateLimitPolicy, rule, key, requests, remaining, resetSeconds(now, current.start, windowSeconds)), nil, nil
	}
	return Decision{
		Allowed:      true,
		QuotaHeaders: quotaHeaders(rateLimitPolicy, requests, remaining, resetSeconds(now, current.start, windowSeconds)),
	}, nil, nil
}

func rejectDecision(policy config.Policy, rule config.Rule, key string, requests, remaining, reset int) Decision {
	return Decision{
		Allowed:      false,
		StatusCode:   policy.RejectedStatusCode(),
		Message:      policy.RejectedMessage(),
		QuotaHeaders: quotaHeaders(policy, requests, remaining, reset),
		Policy:       policy,
		Rule:         rule,
		Key:          key,
	}
}

// ApplyGlobalResult 根据数据面服务返回结果判断是否拒绝请求
func ApplyGlobalResult(checks []GlobalCheck, response dataplaneratelimit.CheckResponse, err error) (Decision, bool) {
	if err != nil || len(response.Results) != len(checks) {
		return rejectFirstFailClose(checks)
	}

	decision := Decision{Allowed: true}
	for i, result := range response.Results {
		check := checks[i]
		if result.Error != "" {
			if check.Policy.FailOpen() {
				continue
			}
			return rejectGlobal(check), true
		}
		next := globalResultDecision(check, result.Allowed, result.Limit, result.Current, result.ResetSeconds)
		if !next.Allowed {
			return next, true
		}
		if len(next.QuotaHeaders) > 0 {
			decision = next
		}
	}
	return decision, false
}

func rejectFirstFailClose(checks []GlobalCheck) (Decision, bool) {
	for _, check := range checks {
		if check.Policy.FailOpen() {
			continue
		}
		return rejectGlobal(check), true
	}
	return Decision{Allowed: true}, false
}

func rejectGlobal(check GlobalCheck) Decision {
	return Decision{
		Allowed:    false,
		StatusCode: check.Policy.RejectedStatusCode(),
		Message:    check.Policy.RejectedMessage(),
		Policy:     check.Policy,
		Rule:       check.Rule,
		Key:        check.Key,
	}
}

func globalResultDecision(check GlobalCheck, allowed bool, requests, current, reset int) Decision {
	remaining := max(requests-current, 0)
	if !allowed {
		return rejectDecision(check.Policy, check.Rule, check.Key, requests, remaining, reset)
	}
	return Decision{
		Allowed:      true,
		QuotaHeaders: quotaHeaders(check.Policy, requests, remaining, reset),
	}
}

func quotaHeaders(policy config.Policy, requests, remaining, reset int) map[string]string {
	if !policy.Response.QuotaHeaderEnabled {
		return nil
	}
	return map[string]string{
		quotaHeaderLimit:     fmt.Sprintf("%d", requests),
		quotaHeaderRemaining: fmt.Sprintf("%d", remaining),
		quotaHeaderReset:     fmt.Sprintf("%d", reset),
	}
}

func resetSeconds(now, start time.Time, windowSeconds int) int {
	resetAt := start.Add(time.Duration(windowSeconds) * time.Second)
	if !resetAt.After(now) {
		return 0
	}
	return int(resetAt.Sub(now).Seconds())
}

func limitKey(route config.RouteConfig, binding config.Binding, policy config.Policy, rule config.Rule, key string) string {
	ruleName := route.RuleName
	if ruleName == "" {
		ruleName = "_"
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s", route.GatewayName, route.RouteName, ruleName, binding.Name, policy.Name, rule.Name) + ":" + key
}
