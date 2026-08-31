package servicemanagement

import (
	"fmt"
	"net"
	"strconv"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
)

func parseEndpoints(configs []*adminv1.ServiceEndpoint) ([]resource.Endpoint, error) {
	if len(configs) == 0 {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"至少需要配置一个服务端点",
		)
	}
	if len(configs) > upstreamconfig.MaxEndpoints {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"服务端点数量超过限制",
		)
	}

	endpoints := make([]resource.Endpoint, 0, len(configs))
	seenEndpointKeys := make(map[string]bool, len(configs))
	for _, config := range configs {
		if config == nil {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"服务端点不能为空",
			)
		}
		endpoint, err := parseEndpoint(config)
		if err != nil {
			return nil, err
		}
		endpointKey := net.JoinHostPort(endpoint.Address, strconv.Itoa(endpoint.Port))
		if seenEndpointKeys[endpointKey] {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				fmt.Sprintf("服务端点 %q 不能重复", endpointKey),
			)
		}
		seenEndpointKeys[endpointKey] = true
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}

func parseEndpoint(config *adminv1.ServiceEndpoint) (resource.Endpoint, error) {
	address := upstreamconfig.NormalizeAddress(config.GetAddress())
	if !upstreamconfig.IsValidAddress(address) {
		return resource.Endpoint{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"服务端点地址格式不正确",
		)
	}
	port := int(config.GetPort())
	if !upstreamconfig.IsValidEndpointPort(port) {
		return resource.Endpoint{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"服务端点端口必须在 1 到 65535 之间",
		)
	}
	weight := int(config.GetWeight())
	if weight == 0 {
		weight = upstreamconfig.DefaultEndpointWeight
	}
	if !upstreamconfig.IsValidEndpointWeight(weight) {
		return resource.Endpoint{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"服务端点权重必须在 1 到 1000 之间",
		)
	}
	return resource.Endpoint{
		Address: address,
		Port:    port,
		Weight:  weight,
	}, nil
}
