package upstream

import (
	"net/url"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func upstreamHealthCheck(input *adminv1.UpstreamHealthCheck) (*resource.UpstreamHealthCheck, error) {
	if input == nil {
		return nil, nil
	}
	path := strings.TrimSpace(input.GetPath())
	if !validHealthCheckPath(path) {
		return nil, adminservice.BadRequest("健康检查路径格式不正确")
	}
	intervalSeconds := int(input.GetIntervalSeconds())
	if intervalSeconds == 0 {
		intervalSeconds = defaultHealthCheckIntervalSeconds
	}
	timeoutSeconds := int(input.GetTimeoutSeconds())
	if timeoutSeconds == 0 {
		timeoutSeconds = defaultHealthCheckTimeoutSeconds
	}
	if timeoutSeconds >= intervalSeconds {
		return nil, adminservice.BadRequest("健康检查间隔或超时时间不正确")
	}
	return &resource.UpstreamHealthCheck{
		Path:            path,
		IntervalSeconds: intervalSeconds,
		TimeoutSeconds:  timeoutSeconds,
	}, nil
}

func validHealthCheckPath(value string) bool {
	if !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "?#") {
		return false
	}
	_, err := url.ParseRequestURI(value)
	return err == nil
}
