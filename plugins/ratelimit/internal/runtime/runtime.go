// Package runtime 编译并执行 RateLimit 插件运行时配置
package runtime

import (
	"errors"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/dataplane"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
)

var ErrDataPlaneUnavailable = errors.New("rate-limit dataplane unavailable")

// Runtime 是 RateLimit 插件配置编译后的请求执行计划
type Runtime struct {
	runner      *policy.Runner
	routes      pluginruntime.RouteIndex[Route]
	redisStores []config.RedisStore
	dataPlane   *dataplane.Client
}

// Route 表示单条 route 的预编译限流配置
type Route struct {
	Config      config.RouteConfig
	HeaderNames []string
}

// Result 表示 RateLimit runtime 对当前请求的处理结果
type Result struct {
	Action       pluginruntime.Action
	QuotaHeaders map[string]string
	GlobalChecks []policy.GlobalCheck
	Errors       []error
}

// Compile 将插件配置转换成请求路径上可直接使用的执行计划
func Compile(cfg config.PluginConfig, runner *policy.Runner) *Runtime {
	routes := make([]Route, 0, len(cfg.Routes))
	for _, routeConfig := range cfg.Routes {
		routes = append(routes, Route{
			Config:      routeConfig,
			HeaderNames: policy.HeaderNames(routeConfig),
		})
	}

	var dataPlane *dataplane.Client
	if cfg.DataPlane != nil {
		client := dataplane.New(*cfg.DataPlane)
		dataPlane = &client
	}

	return &Runtime{
		runner: runner,
		routes: pluginruntime.NewRouteIndex(routes, func(route Route) pluginruntime.RouteKey {
			return pluginruntime.RouteKey{
				GatewayName: route.Config.GatewayName,
				RouteName:   route.Config.RouteName,
				RuleName:    route.Config.RuleName,
			}
		}),
		redisStores: cfg.RedisStores,
		dataPlane:   dataPlane,
	}
}

// Route 返回命中当前 xDS route identity 的限流配置
func (r *Runtime) Route(key pluginruntime.RouteKey) (Route, bool) {
	return r.routes.Get(key)
}

// Apply 对一次请求执行本地限流判断并生成下一步动作
func (r *Runtime) Apply(route Route, request policy.Request) Result {
	result := r.runner.Apply(route.Config, request)
	return resultFromPolicy(result)
}

// DispatchGlobalChecks 发送 global limit 检查请求
func (r *Runtime) DispatchGlobalChecks(checks []policy.GlobalCheck, callback dataplane.CheckCallback) error {
	if r.dataPlane == nil {
		return ErrDataPlaneUnavailable
	}
	return r.dataPlane.CheckGlobal(r.redisStores, checks, callback)
}

// CompleteGlobalChecks 将 dataplane 回调结果转换成插件动作
func (r *Runtime) CompleteGlobalChecks(checks []policy.GlobalCheck, response dataplaneratelimit.CheckResponse, err error) Result {
	decision, rejected := policy.ApplyGlobalResult(checks, response, err)
	if rejected {
		return Result{Action: respondAction(decision)}
	}
	return Result{
		Action:       pluginruntime.Continue(),
		QuotaHeaders: decision.QuotaHeaders,
	}
}

func resultFromPolicy(result policy.Result) Result {
	next := Result{
		Action:       pluginruntime.Continue(),
		QuotaHeaders: result.QuotaHeaders,
		GlobalChecks: result.GlobalChecks,
		Errors:       result.Errors,
	}
	if !result.Allowed {
		next.Action = respondAction(result.Decision)
		return next
	}
	if len(result.GlobalChecks) > 0 {
		next.Action = pluginruntime.Pause()
	}
	return next
}

func respondAction(decision policy.Decision) pluginruntime.Action {
	return pluginruntime.Respond(decision.StatusCode, decision.QuotaHeaders, decision.Message)
}
