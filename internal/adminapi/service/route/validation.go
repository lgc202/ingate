package route

import (
	"net/http"

	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

const defaultRouteTimeoutMillis = 30000

func validateRouteSpec(spec resource.RouteSpec) error {
	if spec.AI != nil && (len(spec.Match.Methods) != 1 || spec.Match.Methods[0] != http.MethodPost) {
		return adminservice.BadRequest("AI 路由目前只支持 POST 请求")
	}
	totalTimeoutMillis := defaultRouteTimeoutMillis
	if spec.Timeout != nil {
		totalTimeoutMillis = spec.Timeout.RequestMillis
	}
	if spec.Retry != nil {
		if spec.Retry.PerTryTimeoutMillis > totalTimeoutMillis {
			return adminservice.BadRequest("单次重试超时不能大于请求总超时")
		}
	}
	return nil
}
