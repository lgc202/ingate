package policy

import (
	"errors"
	"fmt"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

const defaultRedisKeyPrefix = "ingate-rate-limit"

var errLocalCounterUnavailable = errors.New("local rate limit counter unavailable")

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
		return Decision{Allowed: true}, globalCheck(rateLimitPolicy, rule, key, limitKey), nil
	}
	return r.applyLocalRule(rateLimitPolicy, rule, key, limitKey, requests, windowSeconds)
}

func (r *Runner) applyLocalRule(rateLimitPolicy config.Policy, rule config.Rule, key, limitKey string, requests, windowSeconds int) (Decision, *GlobalCheck, error) {
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
	reset := resetSeconds(now, current.start, windowSeconds)
	if current.count > requests {
		return rejectDecision(rateLimitPolicy, rule, key, requests, remaining, reset), nil, nil
	}
	return Decision{
		Allowed:      true,
		QuotaHeaders: quotaHeaders(rateLimitPolicy, requests, remaining, reset),
	}, nil, nil
}

func globalCheck(policy config.Policy, rule config.Rule, key, limitKey string) *GlobalCheck {
	prefix := defaultRedisKeyPrefix
	redisRef := ""
	timeoutMs := 0
	if policy.Global != nil {
		redisRef = policy.Global.RedisRef
		timeoutMs = policy.Global.TimeoutMillis
		if policy.Global.Prefix != "" {
			prefix = policy.Global.Prefix
		}
	}
	return &GlobalCheck{
		Policy:         policy,
		Rule:           rule,
		Key:            key,
		RedisStore:     redisRef,
		RedisKey:       prefix + ":" + limitKey,
		RedisTimeoutMs: timeoutMs,
		Requests:       rule.Limit.Requests,
		WindowSeconds:  rule.Limit.WindowSeconds,
		Burst:          rule.Limit.Burst,
	}
}
