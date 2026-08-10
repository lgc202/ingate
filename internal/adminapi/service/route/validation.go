package route

import (
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultRouteTimeoutMillis = 30000
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
	return nil
}
