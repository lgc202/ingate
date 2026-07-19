package policy

import config "github.com/lgc202/ingate/pkg/plugin/acl"

// Evaluate 对一次请求应用当前 Route 上的全部访问控制策略
func (r Route) Evaluate(req RequestAttributes) Decision {
	for _, policy := range r.policies {
		decision := evaluatePolicy(policy, req)
		if !decision.Allowed {
			return decision
		}
	}
	return Decision{Allowed: true}
}

func evaluatePolicy(policy config.Policy, req RequestAttributes) Decision {
	for _, rule := range policy.Rules {
		if !ruleMatches(rule, req) {
			continue
		}
		if rule.Action == config.ActionAllow {
			return Decision{Allowed: true}
		}
		return denyDecision(policy)
	}
	if policy.DefaultAction == config.ActionDeny {
		return denyDecision(policy)
	}
	return Decision{Allowed: true}
}

func denyDecision(policy config.Policy) Decision {
	return Decision{
		Allowed:    false,
		StatusCode: policy.DeniedStatusCode(),
		Message:    policy.DeniedMessage(),
	}
}
