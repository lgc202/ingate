package policy

import config "github.com/lgc202/ingate/pkg/plugin/acl"

// Apply 对一次请求应用路由 ACL 规则
func (r *Runner) Apply(route config.RouteConfig, req Request) Decision {
	for _, rule := range route.Rules {
		if !ruleMatches(rule, req) {
			continue
		}
		if rule.Action == config.ActionAllow {
			return Decision{Allowed: true, Rule: rule}
		}
		return denyDecision(route, rule)
	}

	if route.DefaultAction == config.ActionDeny {
		return denyDecision(route, config.Rule{Name: "default", Action: config.ActionDeny})
	}
	return Decision{Allowed: true}
}

func denyDecision(route config.RouteConfig, rule config.Rule) Decision {
	return Decision{
		Allowed:    false,
		StatusCode: route.DeniedStatusCode(),
		Message:    route.DeniedMessage(),
		Rule:       rule,
	}
}
