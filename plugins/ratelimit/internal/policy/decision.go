package policy

import (
	"fmt"
	"time"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

const (
	quotaHeaderLimit     = "X-RateLimit-Limit"
	quotaHeaderRemaining = "X-RateLimit-Remaining"
	quotaHeaderReset     = "X-RateLimit-Reset"
)

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
