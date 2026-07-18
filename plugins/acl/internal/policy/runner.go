package policy

import config "github.com/lgc202/ingate/pkg/plugin/acl"

// Apply 对一次请求应用路由 ACL 规则
func (r *Runner) Apply(route config.RouteConfig, req Request) Decision {
	for _, policy := range route.Policies {
		decision := r.applyPolicy(policy, req)
		if !decision.Allowed {
			return decision
		}
	}
	return Decision{Allowed: true}
}

func (r *Runner) applyPolicy(policy config.Policy, req Request) Decision {
	for _, rule := range policy.Rules {
		if !r.ruleMatches(rule, req) {
			continue
		}
		if rule.Action == config.ActionAllow {
			return Decision{Allowed: true}
		}
		return r.denyDecision(policy)
	}
	if policy.DefaultAction == config.ActionDeny {
		return r.denyDecision(policy)
	}
	return Decision{Allowed: true}
}

func (r *Runner) denyDecision(policy config.Policy) Decision {
	return Decision{
		Allowed:    false,
		StatusCode: policy.DeniedStatusCode(),
		Message:    policy.DeniedMessage(),
	}
}
