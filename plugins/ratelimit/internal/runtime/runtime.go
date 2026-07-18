// Package runtime 编译并执行 RateLimit 插件运行时配置
package runtime

import (
	"fmt"
	"time"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"github.com/lgc202/ingate/plugins/internal/redisabi"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/redis"
)

const (
	maxWindowSeconds = int64(^uint64(0)>>1) / int64(time.Second)
)

// Runtime 是 RateLimit 插件配置编译后的请求执行计划
type Runtime struct {
	routes pluginruntime.RouteIndex[Route]
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
	Checks       []policy.Check
}

// Compile 将插件配置转换成请求路径上可直接使用的执行计划
func Compile(cfg config.PluginConfig) *Runtime {
	routes := make([]Route, 0, len(cfg.Routes))
	for _, routeConfig := range cfg.Routes {
		routes = append(routes, Route{
			Config:      routeConfig,
			HeaderNames: policy.HeaderNames(routeConfig),
		})
	}

	return &Runtime{
		routes: pluginruntime.NewRouteIndex(routes, func(route Route) pluginruntime.RouteKey {
			return pluginruntime.RouteKey{
				GatewayName: route.Config.GatewayName,
				RouteName:   route.Config.RouteName,
			}
		}),
	}
}

// Route 返回命中当前 xDS route identity 的限流执行配置
func (r *Runtime) Route(key pluginruntime.RouteKey) (Route, bool) {
	route, ok := r.routes.Get(pluginruntime.RouteKey{
		GatewayName: key.GatewayName,
		RouteName:   key.RouteName,
	})
	if !ok {
		return Route{}, false
	}

	route.RuleName = key.RuleName
	return route, true
}

// Apply 将一次请求展开为共享限流检查并生成下一步动作
func (r *Runtime) Apply(route Route, request policy.Request) Result {
	checks := policy.Checks(route.Config, request)
	result := Result{Action: pluginruntime.Continue(), Checks: checks}
	if len(checks) > 0 {
		result.Action = pluginruntime.Pause()
	}
	return result
}

// PrepareCheck 为一条限流检查生成 Redis 请求和 RESP 命令
func (r *Runtime) PrepareCheck(check policy.Check) (redis.Request, []byte, error) {
	windowSeconds := int64(check.Rule.Limit.WindowSeconds)
	if windowSeconds <= 0 || windowSeconds > maxWindowSeconds {
		return redis.Request{}, nil, fmt.Errorf("rate limit window seconds %d is out of range", windowSeconds)
	}
	request := redis.Request{
		Key:      check.RedisKey,
		Requests: check.Rule.Limit.Requests,
		Window:   time.Duration(windowSeconds) * time.Second,
		Capacity: check.Rule.Limit.Burst,
	}
	if request.Capacity == 0 {
		request.Capacity = request.Requests
	}
	command, err := redis.BuildCommand(request)
	if err != nil {
		return redis.Request{}, nil, err
	}
	return request, command, nil
}

// CompleteCheck 将 Redis ABI callback 转换成统一裁决输入
func (r *Runtime) CompleteCheck(request redis.Request, result redisabi.Result) policy.Outcome {
	if result.Err != nil {
		return policy.Outcome{Err: result.Err}
	}
	if result.Status != redisabi.RedisStatusOK {
		return policy.Outcome{Err: fmt.Errorf("redis call failed with status %d", result.Status)}
	}
	parsed, err := redis.ParseResult(request, result.Data)
	if err != nil {
		return policy.Outcome{Err: err}
	}
	return policy.Outcome{
		Allowed:      parsed.Allowed,
		Current:      parsed.Current,
		Limit:        parsed.Limit,
		ResetSeconds: parsed.ResetSeconds,
	}
}

// CompleteChecks 按原始检查顺序统一生成 Resume 或 Respond 动作
func (r *Runtime) CompleteChecks(checks []policy.Check, outcomes []policy.Outcome) Result {
	decision, rejected := policy.ApplyOutcomes(checks, outcomes)
	if rejected {
		return Result{Action: respondAction(decision)}
	}
	return Result{
		Action:       pluginruntime.Continue(),
		QuotaHeaders: decision.QuotaHeaders,
	}
}

func respondAction(decision policy.Decision) pluginruntime.Action {
	return pluginruntime.Respond(decision.StatusCode, decision.QuotaHeaders, decision.Message)
}
