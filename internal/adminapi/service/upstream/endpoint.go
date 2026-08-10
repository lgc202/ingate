package upstream

import (
	"net"
	"strconv"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func buildEndpoints(inputs []*adminv1.UpstreamEndpoint) ([]resource.Endpoint, error) {
	if len(inputs) == 0 {
		return nil, adminservice.BadRequest("至少需要配置一个服务端点")
	}
	endpoints := make([]resource.Endpoint, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		if input == nil {
			return nil, adminservice.BadRequest("服务端点不能为空")
		}
		address := strings.ToLower(strings.TrimSpace(input.GetAddress()))
		if !adminservice.ValidEndpointAddress(address) {
			return nil, adminservice.BadRequest("服务端点地址格式不正确")
		}
		port := int(input.GetPort())
		if port < 1 || port > 65535 {
			return nil, adminservice.BadRequest("服务端点端口必须在 1 到 65535 之间")
		}
		weight := int(input.GetWeight())
		if weight == 0 {
			weight = defaultEndpointWeight
		}
		if weight > 1000 {
			return nil, adminservice.BadRequest("服务端点权重必须在 1 到 1000 之间")
		}

		key := net.JoinHostPort(address, strconv.Itoa(port))
		if _, exists := seen[key]; exists {
			return nil, adminservice.BadRequest("服务端点不能重复")
		}
		seen[key] = struct{}{}
		endpoints = append(endpoints, resource.Endpoint{Address: address, Port: port, Weight: weight})
	}
	return endpoints, nil
}
