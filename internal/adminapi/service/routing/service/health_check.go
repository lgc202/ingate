package service

import (
	"cmp"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	apivalidation "github.com/lgc202/ingate/internal/pkg/apis/gateway/validation"
)

func parseHealthCheck(
	config *adminv1.ServiceHealthCheck,
) (*resource.UpstreamHealthCheck, error) {
	if config == nil {
		return nil, nil
	}
	path := strings.TrimSpace(config.GetPath())
	if !apivalidation.IsValidHealthCheckPath(path) {
		return nil, adminv1.ErrorInvalidArgument("健康检查路径格式不正确")
	}
	intervalSeconds := cmp.Or(int(config.GetIntervalSeconds()), apivalidation.DefaultHealthCheckIntervalSeconds)
	if !apivalidation.IsValidHealthCheckInterval(intervalSeconds) {
		return nil, adminv1.ErrorInvalidArgument("健康检查间隔必须在 1 到 300 秒之间")
	}
	timeoutSeconds := cmp.Or(int(config.GetTimeoutSeconds()), apivalidation.DefaultHealthCheckTimeoutSeconds)
	if !apivalidation.IsValidHealthCheckTimeout(timeoutSeconds, intervalSeconds) {
		return nil, adminv1.ErrorInvalidArgument("健康检查超时时间必须在 1 到 60 秒之间，且短于检查间隔")
	}
	return &resource.UpstreamHealthCheck{
		Path:            path,
		IntervalSeconds: intervalSeconds,
		TimeoutSeconds:  timeoutSeconds,
	}, nil
}
