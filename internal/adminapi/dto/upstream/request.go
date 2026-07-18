package upstream

import (
	"errors"
	"net/netip"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/bearer"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	minEndpointWeight = 1
	maxEndpointWeight = 100
)

// Validate 校验创建 Upstream 请求
func (r *CreateUpstreamReq) Validate() error {
	if err := r.UpstreamConfig.Validate(); err != nil {
		return err
	}
	return validateAPIKey(r.APIKey, r.Protocol, r.TLS)
}

// Validate 校验更新 Upstream 请求
func (r *UpdateUpstreamReq) Validate() error {
	if r.Version == "" {
		return errors.New("服务版本不能为空")
	}
	if err := r.UpstreamConfig.Validate(); err != nil {
		return err
	}
	if r.APIKey != nil && r.RemoveAPIKey {
		return errors.New("不能同时设置和移除 API Key")
	}
	return validateAPIKey(r.APIKey, r.Protocol, r.TLS)
}

// Validate 校验控制台提交的 Upstream 配置
func (r *UpstreamConfig) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.New("服务名称不能为空")
	}
	if !validServiceType(r.Type) {
		return errors.New("服务类型不正确")
	}
	if !validProtocol(r.Protocol) {
		return errors.New("服务协议不正确")
	}
	if r.Type == resource.UpstreamTypeModel && r.Protocol != resource.UpstreamProtocolOpenAI {
		return errors.New("大模型服务必须使用 OpenAI 兼容协议")
	}
	if r.Type != resource.UpstreamTypeModel && r.Protocol != resource.UpstreamProtocolHTTP {
		return errors.New("当前只有大模型服务支持 OpenAI 兼容协议")
	}
	if r.TLS != nil {
		r.TLS.ServerName = strings.TrimSpace(strings.ToLower(r.TLS.ServerName))
		if !validEndpointAddress(r.TLS.ServerName) {
			return errors.New("HTTPS 服务名称格式不正确")
		}
	}
	if !validLoadBalancePolicy(r.LoadBalancePolicy) {
		return errors.New("负载均衡方式不正确")
	}
	if len(r.Endpoints) == 0 {
		return errors.New("至少需要配置一个服务端点")
	}

	enabledEndpoints := 0
	endpointIDs := make(map[string]bool, len(r.Endpoints))
	for i := range r.Endpoints {
		if err := r.Endpoints[i].Validate(); err != nil {
			return err
		}
		if endpointIDs[r.Endpoints[i].ID] {
			return errors.New("服务端点 ID 不能重复")
		}
		endpointIDs[r.Endpoints[i].ID] = true
		if r.Endpoints[i].Enabled {
			enabledEndpoints++
		}
	}
	if enabledEndpoints == 0 {
		return errors.New("至少需要启用一个服务端点")
	}

	return r.validateHealthCheck()
}

func validateAPIKey(apiKey *APIKeyConfig, protocol resource.UpstreamProtocol, tls *UpstreamTLS) error {
	if apiKey == nil {
		return nil
	}
	if apiKey.Value == "" {
		return errors.New("API Key 不能为空")
	}
	if !bearer.ValidToken(apiKey.Value) {
		return errors.New("API Key 包含不支持的空白字符、控制字符或格式")
	}
	if protocol != resource.UpstreamProtocolOpenAI {
		return errors.New("当前只有 OpenAI 兼容服务支持 API Key")
	}
	if tls == nil {
		return errors.New("配置 API Key 时必须使用 HTTPS")
	}
	return nil
}

// Validate 校验控制台提交的服务端点
func (r *UpstreamEndpoint) Validate() error {
	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		return errors.New("服务端点 ID 不能为空")
	}
	r.Address = strings.TrimSpace(r.Address)
	if r.Address == "" {
		return errors.New("服务端点地址不能为空")
	}
	if !validEndpointAddress(r.Address) {
		return errors.New("服务端点地址格式不正确")
	}

	if r.Port < 1 || r.Port > 65535 {
		return errors.New("服务端点端口必须在 1-65535 之间")
	}

	if r.Weight < minEndpointWeight || r.Weight > maxEndpointWeight {
		return errors.New("服务端点权重必须在 1-100 之间")
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

func validProtocol(value resource.UpstreamProtocol) bool {
	switch value {
	case resource.UpstreamProtocolHTTP, resource.UpstreamProtocolOpenAI:
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
		return errors.New("健康检查路径必须以 / 开头")
	}

	if r.HealthCheck.IntervalSeconds < 1 || r.HealthCheck.IntervalSeconds > 300 {
		return errors.New("健康检查间隔必须在 1-300 秒之间")
	}

	if r.HealthCheck.TimeoutSeconds < 1 || r.HealthCheck.TimeoutSeconds > 60 || r.HealthCheck.TimeoutSeconds >= r.HealthCheck.IntervalSeconds {
		return errors.New("健康检查超时必须在 1-60 秒之间，并且小于检查间隔")
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
