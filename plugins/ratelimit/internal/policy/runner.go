package policy

import (
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

const defaultRedisKeyPrefix = "ingate-rate-limit"

// Checks 将当前请求展开为系统 Redis 限流检查
func Checks(route config.RouteConfig, req Request) []Check {
	var checks []Check
	for _, rateLimitPolicy := range route.Policies {
		for _, rule := range rateLimitPolicy.Rules {
			check := buildCheck(rateLimitPolicy, rule, req)
			if check != nil {
				checks = append(checks, *check)
			}
		}
	}
	return checks
}

func buildCheck(rateLimitPolicy config.Policy, rule config.Rule, req Request) *Check {
	compositeHash, ok := compositeKeyHash(req, rule.Key)
	if !ok {
		return nil
	}

	if rule.Limit.Requests <= 0 || rule.Limit.WindowSeconds <= 0 {
		return nil
	}

	return newCheck(rateLimitPolicy, rule, limitKey(rateLimitPolicy, rule, compositeHash))
}

func newCheck(policy config.Policy, rule config.Rule, redisKey string) *Check {
	return &Check{
		Policy:   policy,
		Rule:     rule,
		RedisKey: redisKey,
	}
}
