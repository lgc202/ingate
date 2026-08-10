package route

import (
	"net/http"
	"strings"

	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultRouteTimeoutMillis = 30000
	openAIChatCompletionsPath = "/v1/chat/completions"
	aiClusterHeader           = "x-ingate-ai-cluster-v1"
)

func validateRouteBehavior(spec resource.RouteSpec) error {
	totalTimeoutMillis := defaultRouteTimeoutMillis
	if spec.Timeout != nil {
		totalTimeoutMillis = spec.Timeout.RequestMillis
		if totalTimeoutMillis < 100 || totalTimeoutMillis > 300000 {
			return adminservice.BadRequest("请求超时必须在 100 到 300000 毫秒之间")
		}
	}
	if spec.Retry != nil {
		if spec.Retry.Attempts < 1 || spec.Retry.Attempts > 5 ||
			spec.Retry.PerTryTimeoutMillis < 100 || spec.Retry.PerTryTimeoutMillis > 60000 {
			return adminservice.BadRequest("重试配置不正确")
		}
		if spec.Retry.PerTryTimeoutMillis > totalTimeoutMillis {
			return adminservice.BadRequest("单次重试超时不能大于请求总超时")
		}
	}
	if spec.ModelRouting == nil {
		return nil
	}
	if spec.Match.Path.Type != resource.PathMatchExact || spec.Match.Path.Value != openAIChatCompletionsPath ||
		len(spec.Match.Methods) != 1 || spec.Match.Methods[0] != http.MethodPost {
		return adminservice.BadRequest("模型路由必须精确匹配 POST /v1/chat/completions")
	}
	if spec.Retry != nil {
		return adminservice.BadRequest("模型路由暂不支持重试")
	}
	for _, header := range spec.Match.Headers {
		if strings.EqualFold(header.Name, aiClusterHeader) {
			return adminservice.BadRequest("模型路由不能匹配系统内部 Header")
		}
	}
	if containsManagedHeader(spec.RequestHeaderModifier) {
		return adminservice.BadRequest("模型路由的请求 Header 修改不能使用系统管理的名称")
	}
	return nil
}
