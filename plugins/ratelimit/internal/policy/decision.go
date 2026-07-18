package policy

import (
	"fmt"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

const (
	quotaHeaderLimit     = "X-RateLimit-Limit"
	quotaHeaderRemaining = "X-RateLimit-Remaining"
	quotaHeaderReset     = "X-RateLimit-Reset"
)

func rejectDecision(policy config.Policy, requests, remaining, reset int) Decision {
	return Decision{
		Allowed:      false,
		StatusCode:   policy.RejectedStatusCode(),
		Message:      policy.RejectedMessage(),
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

func limitKey(policy config.Policy, rule config.Rule, compositeHash string) string {
	return encodeKeySegments(
		defaultRedisKeyPrefix,
		policy.Name,
		policy.Scope,
		rule.Name,
		compositeHash,
	)
}
