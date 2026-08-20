package upstream

import (
	"net"
	"net/netip"
	"strconv"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func upstreamEndpoints(inputs []*adminv1.UpstreamEndpoint) ([]resource.Endpoint, error) {
	endpoints := make([]resource.Endpoint, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		if input == nil {
			return nil, adminservice.BadRequest("服务端点不能为空")
		}
		address := strings.ToLower(strings.TrimSpace(input.GetAddress()))
		if !validEndpointAddress(address) {
			return nil, adminservice.BadRequest("服务端点地址格式不正确")
		}
		port := int(input.GetPort())
		weight := int(input.GetWeight())
		if weight == 0 {
			weight = defaultEndpointWeight
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

func validEndpointAddress(address string) bool {
	if _, err := netip.ParseAddr(address); err == nil {
		return true
	}
	if address == "" || len(address) > 253 {
		return false
	}
	for label := range strings.SplitSeq(strings.ToLower(address), ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}
