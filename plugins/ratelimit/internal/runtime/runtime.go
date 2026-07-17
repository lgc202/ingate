// Package runtime 编译并执行 RateLimit 插件运行时配置
package runtime

import (
	"fmt"
	"sync/atomic"
	"time"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"github.com/lgc202/ingate/plugins/internal/redisabi"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/redis"
)

const (
	bindingTargetGateway = "Gateway"
	bindingTargetRoute   = "Route"
	maxWindowSeconds     = int64(^uint64(0)>>1) / int64(time.Second)
)

// Runtime 是 RateLimit 插件配置编译后的请求执行计划
type Runtime struct {
	runner         *policy.Runner
	routes         pluginruntime.RouteIndex[Route]
	memberSequence atomic.Uint64
}

// Route 表示单条 route 的预编译限流配置
type Route struct {
	Config      config.RouteConfig
	RuleName    string
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

// Route 返回命中当前 xDS route identity 且已经按 rule scope 过滤的限流配置
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
	route.RuleName = key.RuleName
	route.HeaderNames = policy.HeaderNames(route.Config)
	return route, true
}

// Apply 对一次请求执行本地限流判断并生成下一步动作
func (r *Runtime) Apply(route Route, request policy.Request) Result {
	result := r.runner.Apply(route.Config, request)
	return resultFromPolicy(result)
}

// PrepareGlobalCheck 为一条 global limit 检查生成 Redis 请求和 RESP 命令
func (r *Runtime) PrepareGlobalCheck(check policy.GlobalCheck) (redis.Request, []byte, error) {
	now := time.Now()
	windowSeconds := int64(check.Rule.Limit.WindowSeconds)
	if windowSeconds <= 0 || windowSeconds > maxWindowSeconds {
		return redis.Request{}, nil, fmt.Errorf("rate limit window seconds %d is out of range", windowSeconds)
	}
	request := redis.Request{
		Algorithm: redis.Algorithm(check.Rule.Algorithm),
		Key:       check.RedisKey,
		Requests:  check.Rule.Limit.Requests,
		Window:    time.Duration(windowSeconds) * time.Second,
		Burst:     check.Rule.Limit.Burst,
		Now:       now,
		Member:    fmt.Sprintf("%d-%d", now.UnixNano(), r.memberSequence.Add(1)),
	}
	command, err := redis.BuildCommand(request)
	if err != nil {
		return redis.Request{}, nil, err
	}
	return request, command, nil
}

// CompleteGlobalCheck 将 Redis ABI callback 转换成统一裁决输入
func (r *Runtime) CompleteGlobalCheck(request redis.Request, result redisabi.Result) policy.GlobalOutcome {
	if result.Err != nil {
		return policy.GlobalOutcome{Err: result.Err}
	}
	if result.Status != redisabi.RedisStatusOK {
		return policy.GlobalOutcome{Err: fmt.Errorf("redis call failed with status %d", result.Status)}
	}
	parsed, err := redis.ParseResult(request, result.Data)
	if err != nil {
		return policy.GlobalOutcome{Err: err}
	}
	return policy.GlobalOutcome{
		Allowed:      parsed.Allowed,
		Current:      parsed.Current,
		Limit:        parsed.Limit,
		ResetSeconds: parsed.ResetSeconds,
	}
}

// CompleteGlobalChecks 按原始检查顺序统一生成 Resume 或 Respond 动作
func (r *Runtime) CompleteGlobalChecks(checks []policy.GlobalCheck, outcomes []policy.GlobalOutcome) Result {
	decision, rejected := policy.ApplyGlobalResults(checks, outcomes)
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
