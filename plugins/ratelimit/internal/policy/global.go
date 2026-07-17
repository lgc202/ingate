package policy

// ApplyGlobalResults 按原始策略顺序统一裁决串行 Redis 检查结果
func ApplyGlobalResults(checks []GlobalCheck, outcomes []GlobalOutcome) (Decision, bool) {
	decision := Decision{Allowed: true}
	for i, check := range checks {
		if i >= len(outcomes) || outcomes[i].Err != nil {
			if check.Policy.FailOpen() {
				continue
			}
			return rejectGlobal(check), true
		}

		outcome := outcomes[i]
		next := globalResultDecision(check, outcome)
		if !next.Allowed {
			return next, true
		}
		if len(next.QuotaHeaders) > 0 {
			decision = next
		}
	}
	return decision, false
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

func globalResultDecision(check GlobalCheck, outcome GlobalOutcome) Decision {
	remaining := max(outcome.Limit-outcome.Current, 0)
	if !outcome.Allowed {
		return rejectDecision(
			check.Policy,
			check.Rule,
			check.Key,
			outcome.Limit,
			remaining,
			outcome.ResetSeconds,
		)
	}
	return Decision{
		Allowed:      true,
		QuotaHeaders: quotaHeaders(check.Policy, outcome.Limit, remaining, outcome.ResetSeconds),
	}
}
