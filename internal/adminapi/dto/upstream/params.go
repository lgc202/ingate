package upstream

import (
	"github.com/samber/lo"

	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
)

// Params 将已校验的创建请求转换为 service 参数
func (r CreateUpstreamReq) Params() upstreamservice.CreateUpstreamParams {
	return upstreamservice.CreateUpstreamParams{
		UpstreamParams: r.UpstreamConfig.params(),
	}
}

// Params 将已校验的更新请求转换为 service 参数
func (r UpdateUpstreamReq) Params() upstreamservice.UpdateUpstreamParams {
	return upstreamservice.UpdateUpstreamParams{
		Version:        r.Version,
		UpstreamParams: r.UpstreamConfig.params(),
	}
}

func (c UpstreamConfig) params() upstreamservice.UpstreamParams {
	return upstreamservice.UpstreamParams{
		Name:              c.Name,
		Type:              c.Type,
		Protocol:          c.Protocol,
		TLS:               tlsParams(c.TLS),
		CredentialID:      c.CredentialID,
		LoadBalancePolicy: c.LoadBalancePolicy,
		Endpoints: lo.Map(c.Endpoints, func(endpoint UpstreamEndpoint, _ int) upstreamservice.EndpointParams {
			return upstreamservice.EndpointParams{
				ID:      endpoint.ID,
				Address: endpoint.Address,
				Port:    endpoint.Port,
				Weight:  endpoint.Weight,
				Enabled: endpoint.Enabled,
			}
		}),
		HealthCheck: c.HealthCheck,
	}
}

func tlsParams(config *UpstreamTLS) *upstreamservice.TLSParams {
	if config == nil {
		return nil
	}
	return &upstreamservice.TLSParams{ServerName: config.ServerName}
}
