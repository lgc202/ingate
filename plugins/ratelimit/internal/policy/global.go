package policy

import dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"

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
