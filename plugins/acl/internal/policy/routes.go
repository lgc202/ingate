package policy

import config "github.com/lgc202/ingate/pkg/plugin/acl"

// NewRoutes 为 Listener 中的访问控制配置建立 Route 索引
func NewRoutes(cfg config.PluginConfig) Routes {
	routes := make(Routes, len(cfg.Routes))
	for _, item := range cfg.Routes {
		routes[RouteKey{GatewayName: item.GatewayName, RouteName: item.RouteName}] = Route{
			policies:        item.Policies,
			RequiredHeaders: requiredHeaders(item),
		}
	}
	return routes
}
