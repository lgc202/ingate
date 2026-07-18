package policy

// ApplyOutcomes 按原始策略顺序统一裁决串行 Redis 检查结果
func ApplyOutcomes(checks []Check, outcomes []Outcome) (Decision, bool) {
	decision := Decision{Allowed: true}
	strictestRemaining := 0
	hasQuotaHeaders := false
	for i, check := range checks {
		if i >= len(outcomes) || outcomes[i].Err != nil {
			if check.Policy.FailOpen() {
				continue
			}
			return rejectCheck(check), true
		}

		next := outcomeDecision(check, outcomes[i])
		if !next.Allowed {
			return next, true
		}
		remaining := max(outcomes[i].Limit-outcomes[i].Current, 0)
		if len(next.QuotaHeaders) > 0 && (!hasQuotaHeaders || remaining < strictestRemaining) {
			decision = next
			strictestRemaining = remaining
			hasQuotaHeaders = true
		}
	}
	return decision, false
}

func rejectCheck(check Check) Decision {
	return Decision{
		Allowed:    false,
		StatusCode: check.Policy.RejectedStatusCode(),
		Message:    check.Policy.RejectedMessage(),
	}
}

func outcomeDecision(check Check, outcome Outcome) Decision {
	remaining := max(outcome.Limit-outcome.Current, 0)
	if !outcome.Allowed {
		return rejectDecision(
			check.Policy,
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
