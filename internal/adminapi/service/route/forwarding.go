package route

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func buildForwarding(spec *resource.RouteSpec, upstreams []*adminv1.RouteUpstream) error {
	if len(upstreams) == 0 {
		return adminservice.BadRequest("至少需要配置一个目标服务")
	}

	seen := make(map[string]struct{}, len(upstreams))
	for _, input := range upstreams {
		if input == nil {
			return adminservice.BadRequest("目标服务不能为空")
		}
		id := strings.TrimSpace(input.GetUpstreamId())
		if id == "" || input.GetWeight() < 1 || input.GetWeight() > 1000 {
			return adminservice.BadRequest("目标服务 ID 或权重不正确")
		}
		if _, exists := seen[id]; exists {
			return adminservice.BadRequest("目标服务不能重复")
		}
		seen[id] = struct{}{}
		spec.UpstreamRefs = append(spec.UpstreamRefs, resource.UpstreamRef{Name: id, Weight: int(input.GetWeight())})
	}
	return nil
}
