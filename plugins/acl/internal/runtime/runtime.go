// Package runtime 编译并执行 ACL 插件运行时配置
package runtime

import (
	config "github.com/lgc202/ingate/pkg/plugin/acl"
	"github.com/lgc202/ingate/plugins/acl/internal/policy"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
)

const (
	bindingTargetGateway = "Gateway"
	bindingTargetRoute   = "Route"
)

// Runtime 是 ACL 插件配置编译后的请求执行计划
type Runtime struct {
	runner *policy.Runner
	routes pluginruntime.RouteIndex[Route]
}

// Route 表示单条 route 的预编译 ACL 配置
type Route struct {
	Config      config.RouteConfig
	HeaderNames []string
}

// Compile 将插件配置转换成请求路径上可直接使用的执行计划
func Compile(cfg config.PluginConfig, runner *policy.Runner) *Runtime {
	routes := make([]Route, 0, len(cfg.Routes))
	for _, routeConfig := range cfg.Routes {
		routes = append(routes, Route{Config: routeConfig})
	}

	return &Runtime{
		runner: runner,
		routes: pluginruntime.NewRouteIndex(routes, func(route Route) pluginruntime.RouteKey {
			return pluginruntime.RouteKey{
				GatewayName: route.Config.GatewayName,
				RouteName:   route.Config.RouteName,
			}
		}),
	}
}

// Route 返回命中当前 xDS route identity 且已经按 rule scope 过滤的 ACL 配置
func (r *Runtime) Route(key pluginruntime.RouteKey) (Route, bool) {
	route, ok := r.routes.Get(pluginruntime.RouteKey{
		GatewayName: key.GatewayName,
		RouteName:   key.RouteName,
	})
	if !ok {
		return Route{}, false
	}

	bindings := make([]config.Binding, 0, len(route.Config.Bindings))
	for _, binding := range route.Config.Bindings {
		switch binding.Target.Kind {
		case bindingTargetGateway:
			if binding.Target.Name == key.GatewayName {
				bindings = append(bindings, binding)
			}
		case bindingTargetRoute:
			if binding.Target.Name == key.RouteName && (binding.Target.RuleName == "" || binding.Target.RuleName == key.RuleName) {
				bindings = append(bindings, binding)
			}
		}
	}
	route.Config.Bindings = bindings
	route.HeaderNames = policy.HeaderNames(route.Config)
	return route, true
}

// Apply 对一次请求执行 ACL 判断并生成插件动作
func (r *Runtime) Apply(route Route, request policy.Request) pluginruntime.Action {
	decision := r.runner.Apply(route.Config, request)
	if decision.Allowed {
		return pluginruntime.Continue()
	}
	return pluginruntime.Respond(decision.StatusCode, nil, decision.Message)
}
