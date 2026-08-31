package servicemanagement

import (
	"strings"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
)

func parseHealthCheck(
	config *adminv1.ServiceHealthCheck,
) (*resource.UpstreamHealthCheck, error) {
	if config == nil {
		return nil, nil
	}
	path := strings.TrimSpace(config.GetPath())
	if !upstreamconfig.IsValidHealthCheckPath(path) {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"健康检查路径格式不正确",
		)
	}
	intervalSeconds := int(config.GetIntervalSeconds())
	if intervalSeconds == 0 {
		intervalSeconds = upstreamconfig.DefaultHealthCheckIntervalSeconds
	}
	if !upstreamconfig.IsValidHealthCheckInterval(intervalSeconds) {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"健康检查间隔必须在 1 到 300 秒之间",
		)
	}
	timeoutSeconds := int(config.GetTimeoutSeconds())
	if timeoutSeconds == 0 {
		timeoutSeconds = upstreamconfig.DefaultHealthCheckTimeoutSeconds
	}
	if !upstreamconfig.IsValidHealthCheckTimeout(timeoutSeconds, intervalSeconds) {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"健康检查超时时间必须在 1 到 60 秒之间，且短于检查间隔",
		)
	}
	return &resource.UpstreamHealthCheck{
		Path:            path,
		IntervalSeconds: intervalSeconds,
		TimeoutSeconds:  timeoutSeconds,
	}, nil
}
