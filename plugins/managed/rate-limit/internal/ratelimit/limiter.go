package ratelimit

import (
	"errors"
	"fmt"
	"time"

	"github.com/lgc202/ingate/plugins/managed/rate-limit/internal/config"
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

// Result 表示一次请求经过本地规则后的结果
type Result struct {
	Allowed      bool
	Decision     Decision
	QuotaHeaders map[string]string
	RedisChecks  []RedisCheck
	Errors       []error
}

// RedisCheck 表示需要交给 Redis 执行的 global limit 检查
type RedisCheck struct {
	Policy         config.Policy
	Rule           config.Rule
	Key            string
	RedisStore     string
	RedisKey       string
	RedisTimeoutMs int
	Requests       int
	WindowSeconds  int
}

// LocalLimiter 执行本地 fixed-window 限流
type LocalLimiter struct {
	store counterStore
	now   func() time.Time
}

type window struct {
	start time.Time
	count int
}

func NewLocalLimiter() *LocalLimiter {
	return &LocalLimiter{
		store: newMemoryCounterStore(),
		now:   time.Now,
	}
}

func NewLocalLimiterWithClock(now func() time.Time) *LocalLimiter {
	return &LocalLimiter{
		store: newMemoryCounterStore(),
		now:   now,
	}
}

// NewSharedDataLocalLimiter 使用 Envoy shared data 保存本地限流计数
func NewSharedDataLocalLimiter() *LocalLimiter {
	return &LocalLimiter{
		store: sharedDataCounterStore{},
		now:   time.Now,
	}
}

func (l *LocalLimiter) Evaluate(route config.RouteConfig, req Request) Result {
	result := Result{Allowed: true}
	for _, binding := range route.Bindings {
		for _, policy := range binding.Policies {
			for _, rule := range policy.Rules {
				decision, redisCheck, err := l.evaluateRule(route, binding, policy, rule, req)
				if err != nil {
					result.Errors = append(result.Errors, err)
				}
				if !decision.Allowed {
					result.Allowed = false
					result.Decision = decision
					return result
				}
				if redisCheck != nil {
					result.RedisChecks = append(result.RedisChecks, *redisCheck)
				}
				if len(decision.QuotaHeaders) > 0 {
					result.QuotaHeaders = decision.QuotaHeaders
				}
			}
		}
	}
	return result
}

func (l *LocalLimiter) evaluateRule(route config.RouteConfig, binding config.Binding, policy config.Policy, rule config.Rule, req Request) (Decision, *RedisCheck, error) {
	key, ok := compositeKey(req, rule.Key)
	if !ok {
		return Decision{Allowed: true}, nil, nil
	}

	requests := rule.Limit.Requests
	windowSeconds := rule.Limit.WindowSeconds
	if requests <= 0 || windowSeconds <= 0 {
		return Decision{Allowed: true}, nil, nil
	}

	limitKey := limitKey(route, binding, policy, rule, key)
	if policy.Mode == config.ModeGlobal {
		redisRef := ""
		timeoutMs := 0
		prefix := "ingate-rate-limit"
		if policy.Global != nil {
			redisRef = policy.Global.RedisRef
			timeoutMs = policy.Global.TimeoutMillis
			if policy.Global.Prefix != "" {
				prefix = policy.Global.Prefix
			}
		}
		return Decision{Allowed: true}, &RedisCheck{
			Policy:         policy,
			Rule:           rule,
			Key:            key,
			RedisStore:     redisRef,
			RedisKey:       prefix + ":" + limitKey,
			RedisTimeoutMs: timeoutMs,
			Requests:       requests,
			WindowSeconds:  windowSeconds,
		}, nil
	}

	now := l.now()
	current, err := l.store.Increment(limitKey, now, windowSeconds)
	if err != nil {
		storeErr := fmt.Errorf("%w: %s", errLocalCounterUnavailable, err)
		if policy.FailOpen() {
			return Decision{Allowed: true}, nil, storeErr
		}
		return rejectDecision(policy, rule, key, requests, 0, windowSeconds), nil, storeErr
	}

	remaining := requests - current.count
	if remaining < 0 {
		remaining = 0
	}
	if current.count > requests {
		return rejectDecision(policy, rule, key, requests, remaining, resetSeconds(now, current.start, windowSeconds)), nil, nil
	}
	return Decision{
		Allowed:      true,
		QuotaHeaders: quotaHeaders(policy, requests, remaining, resetSeconds(now, current.start, windowSeconds)),
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

func RedisDecision(policy config.Policy, rule config.Rule, key string, requests, current, reset int) Decision {
	remaining := requests - current
	if remaining < 0 {
		remaining = 0
	}
	if current > requests {
		return rejectDecision(policy, rule, key, requests, remaining, reset)
	}
	return Decision{
		Allowed:      true,
		QuotaHeaders: quotaHeaders(policy, requests, remaining, reset),
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
