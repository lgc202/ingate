package dto

import (
	"errors"
	"net/netip"
	"strings"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	minEndpointWeight = 1
	maxEndpointWeight = 100
)

// Validate 校验创建 Upstream 请求
func (r *CreateUpstreamReq) Validate() error {
	return r.UpstreamConfig.Validate()
}

// Validate 校验更新 Upstream 请求
func (r *UpdateUpstreamReq) Validate() error {
	if r.Version == "" {
		return errors.New("upstream version is required")
	}
	return r.UpstreamConfig.Validate()
}

// Validate 校验控制台提交的 Upstream 配置
func (r *UpstreamConfig) Validate() error {
	if r.Name == "" {
		return errors.New("service name is required")
	}
	if !validServiceType(r.Type) {
		return errors.New("service type is invalid")
	}
	if !validLoadBalancePolicy(r.LoadBalancePolicy) {
		return errors.New("load balance policy is invalid")
	}
	if len(r.Endpoints) == 0 {
		return errors.New("at least one service endpoint is required")
	}

	enabledEndpoints := 0
	for i := range r.Endpoints {
		if err := r.Endpoints[i].Validate(); err != nil {
			return err
		}
		if r.Endpoints[i].Enabled {
			enabledEndpoints++
		}
	}
	if enabledEndpoints == 0 {
		return errors.New("at least one service endpoint must be enabled")
	}

	return r.validateHealthCheck()
}

// Validate 校验控制台提交的服务端点
func (r *UpstreamEndpoint) Validate() error {
	if r.ID == "" {
		return errors.New("endpoint id is required")
	}
	if r.Address == "" {
		return errors.New("endpoint address is required")
	}
	if !validEndpointAddress(r.Address) {
		return errors.New("endpoint address is invalid")
	}

	if r.Port < 1 || r.Port > 65535 {
		return errors.New("endpoint port must be between 1 and 65535")
	}

	if r.Weight < minEndpointWeight || r.Weight > maxEndpointWeight {
		return errors.New("endpoint weight must be between 1 and 100")
	}

	return nil
}

func validServiceType(value resource.UpstreamType) bool {
	switch value {
	case resource.UpstreamTypeApplication, resource.UpstreamTypeModel, resource.UpstreamTypeAgent, resource.UpstreamTypeMCP:
		return true
	default:
		return false
	}
}

func validLoadBalancePolicy(value resource.UpstreamLoadBalancePolicy) bool {
	switch value {
	case resource.UpstreamLoadBalancePolicyRoundRobin, resource.UpstreamLoadBalancePolicyLeastRequest, resource.UpstreamLoadBalancePolicyRandom:
		return true
	default:
		return false
	}
}

func validEndpointAddress(address string) bool {
	if _, err := netip.ParseAddr(address); err == nil {
		return true
	}
	return validHostname(strings.ToLower(address))
}

func (r *UpstreamConfig) validateHealthCheck() error {
	if r.HealthCheck == nil || !r.HealthCheck.Enabled {
		return nil
	}

	if !strings.HasPrefix(r.HealthCheck.Path, "/") {
		return errors.New("health check path must start with /")
	}

	if r.HealthCheck.IntervalSeconds < 1 || r.HealthCheck.IntervalSeconds > 300 {
		return errors.New("health check interval must be between 1 and 300")
	}

	if r.HealthCheck.TimeoutSeconds < 1 || r.HealthCheck.TimeoutSeconds > 60 || r.HealthCheck.TimeoutSeconds >= r.HealthCheck.IntervalSeconds {
		return errors.New("health check timeout must be between 1 and 60 and less than interval")
	}

	return nil
}

func validHostname(hostname string) bool {
	if hostname == "" || len(hostname) > 253 {
		return false
	}
	for label := range strings.SplitSeq(hostname, ".") {
		if !validDNSLabel(label) {
			return false
		}
	}
	return true
}

func validDNSLabel(label string) bool {
	if label == "" || len(label) > 63 {
		return false
	}
	for i, r := range label {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-'
		if !valid {
			return false
		}
		if (i == 0 || i == len(label)-1) && r == '-' {
			return false
		}
	}
	return true
}
