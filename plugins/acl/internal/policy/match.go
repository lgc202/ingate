package policy

import (
	"net"
	"net/netip"
	"strings"

	config "github.com/lgc202/ingate/pkg/plugin/acl"
)

func requiredHeaders(route config.RouteConfig) []string {
	seen := map[string]struct{}{}
	for _, policy := range route.Policies {
		for _, rule := range policy.Rules {
			for _, condition := range rule.Conditions {
				if condition.Type == config.ConditionTypeHeader && condition.Name != "" {
					seen[condition.Name] = struct{}{}
				}
			}
		}
	}

	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	return result
}

func ruleMatches(rule config.Rule, req RequestAttributes) bool {
	for _, condition := range rule.Conditions {
		if !conditionMatches(condition, req) {
			return false
		}
	}
	return true
}

func conditionMatches(condition config.Condition, req RequestAttributes) bool {
	switch condition.Type {
	case config.ConditionTypeIP:
		return ipMatches(clientIP(req.RemoteAddr), condition.Value)
	case config.ConditionTypeHeader:
		return headerValue(req.Headers, condition.Name) == condition.Value
	default:
		return false
	}
}

func ipMatches(value, pattern string) bool {
	if value == "" || pattern == "" {
		return false
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return value == pattern
	}
	if prefix, err := netip.ParsePrefix(pattern); err == nil {
		return prefix.Contains(addr)
	}
	return value == pattern
}

func headerValue(headers map[string]string, name string) string {
	if name == "" {
		return ""
	}
	if value := headers[name]; value != "" {
		return value
	}
	lowerName := strings.ToLower(name)
	for key, value := range headers {
		if strings.ToLower(key) == lowerName {
			return value
		}
	}
	return ""
}

func clientIP(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return strings.Trim(host, "[]")
	}
	if value, _, ok := strings.Cut(remoteAddr, ","); ok {
		return strings.TrimSpace(value)
	}
	return strings.Trim(remoteAddr, "[]")
}
