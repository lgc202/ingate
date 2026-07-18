// Package runtime 编译并执行 AI Proxy 插件运行时配置
package runtime

import (
	"encoding/json"

	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
	"github.com/lgc202/ingate/plugins/aiproxy/internal/policy"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
)

const (
	// MaxRequestBodyBytes 限制第一版文本请求的缓冲大小，避免为共享 Listener 扩大内存边界
	MaxRequestBodyBytes = 1 << 20
	jsonContentType     = "application/json"
)

type errorEnvelope struct {
	Error errorResponse `json:"error"`
}

type errorResponse struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    string  `json:"code"`
}

// Runtime 是 AI Proxy 插件配置编译后的请求执行计划
type Runtime struct {
	runner *policy.Runner
	routes pluginruntime.RouteIndex[Route]
}

// Route 表示单条 RouteRule 的预编译模型映射
type Route struct {
	Config config.RouteConfig
	Models map[string]config.ModelConfig
}

// Result 表示 AI Proxy runtime 对当前请求的处理结果
type Result struct {
	Action   pluginruntime.Action
	Mutation policy.Mutation
}

// Compile 将插件配置转换成请求路径上可直接使用的执行计划
func Compile(cfg config.PluginConfig, runner *policy.Runner) *Runtime {
	routes := make([]Route, 0, len(cfg.Routes))
	for _, routeConfig := range cfg.Routes {
		models := make(map[string]config.ModelConfig, len(routeConfig.Models))
		for _, model := range routeConfig.Models {
			models[model.Model] = model
		}
		routes = append(routes, Route{Config: routeConfig, Models: models})
	}

	return &Runtime{
		runner: runner,
		routes: pluginruntime.NewRouteIndex(routes, func(route Route) pluginruntime.RouteKey {
			return pluginruntime.RouteKey{
				GatewayName: route.Config.GatewayName,
				RouteName:   route.Config.RouteName,
				RuleName:    route.Config.RuleName,
				ConfigID:    route.Config.ConfigID,
			}
		}),
	}
}

// Route 返回命中当前 xDS route identity 的 AI Proxy 执行配置
func (r *Runtime) Route(key pluginruntime.RouteKey) (Route, bool) {
	return r.routes.Get(key)
}

// ValidateEndpoint 校验请求是否属于第一版支持的 OpenAI API
func (r *Runtime) ValidateEndpoint(method, path string) Result {
	rejection := r.runner.ValidateEndpoint(policy.Request{Method: method, Path: path})
	if rejection != nil {
		return Result{Action: rejectionAction(rejection)}
	}
	return Result{Action: pluginruntime.Continue()}
}

// Apply 根据请求体选择模型并生成请求改写结果
func (r *Runtime) Apply(route Route, body []byte) Result {
	decision := r.runner.Apply(route.Models, policy.Request{Body: body})
	if decision.Rejection != nil {
		return Result{Action: rejectionAction(decision.Rejection)}
	}
	return Result{
		Action:   pluginruntime.Continue(),
		Mutation: decision.Mutation,
	}
}

// RequestTooLarge 返回请求体超过插件缓冲上限时的本地响应
func (r *Runtime) RequestTooLarge() Result {
	return Result{Action: rejectionAction(r.runner.RequestTooLarge())}
}

// InternalError 返回插件无法完成请求改写时的本地响应
func (r *Runtime) InternalError() Result {
	return Result{Action: rejectionAction(r.runner.InternalError())}
}

func rejectionAction(rejection *policy.Rejection) pluginruntime.Action {
	headers := map[string]string{"content-type": jsonContentType}
	if rejection.Allow != "" {
		headers["allow"] = rejection.Allow
	}
	body, err := json.Marshal(errorEnvelope{
		Error: errorResponse{
			Message: rejection.Message,
			Type:    rejection.Type,
			Param:   optionalString(rejection.Param),
			Code:    rejection.Code,
		},
	})
	if err != nil {
		body = []byte(`{"error":{"message":"The request could not be processed","type":"server_error","param":null,"code":"internal_error"}}`)
	}
	return pluginruntime.Respond(rejection.StatusCode, headers, string(body))
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
