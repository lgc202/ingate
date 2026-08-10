package policy

import (
	"fmt"
	"net/netip"

	config "github.com/lgc202/ingate/pkg/plugin/iprestriction"
)

// NewRoutes 为 Listener 中的 IP 访问限制配置建立 Route 索引
func NewRoutes(cfg config.PluginConfig) (Routes, error) {
	routes := make(Routes, len(cfg.Routes))
	for _, item := range cfg.Routes {
		policies := make([]restriction, 0, len(item.Policies))
		for _, policy := range item.Policies {
			restriction, err := newRestriction(policy)
			if err != nil {
				return nil, err
			}
			policies = append(policies, restriction)
		}
		routes[RouteKey{GatewayName: item.GatewayName, RouteName: item.RouteName}] = Route{policies: policies}
	}
	return routes, nil
}

func newRestriction(policy config.Policy) (restriction, error) {
	if (len(policy.Allow) > 0) == (len(policy.Deny) > 0) {
		return restriction{}, fmt.Errorf("IP restriction policy %q must configure exactly one of allow or deny", policy.Name)
	}
	allow, err := parsePrefixes(policy.Allow)
	if err != nil {
		return restriction{}, fmt.Errorf("parse allow list for IP restriction policy %q: %w", policy.Name, err)
	}
	deny, err := parsePrefixes(policy.Deny)
	if err != nil {
		return restriction{}, fmt.Errorf("parse deny list for IP restriction policy %q: %w", policy.Name, err)
	}
	return restriction{allow: allow, deny: deny}, nil
}

func parsePrefixes(values []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("parse IP prefix %q: %w", value, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}
