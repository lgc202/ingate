package upstream

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// Spec 将已校验的创建请求转换为声明式 UpstreamSpec
func (r CreateUpstreamReq) Spec() resource.UpstreamSpec {
	spec := r.UpstreamConfig.spec()
	spec.Authentication = authenticationFromAPIKey(r.APIKey)
	return spec
}

// Spec 将已校验的更新请求转换为声明式 UpstreamSpec
func (r UpdateUpstreamReq) Spec() resource.UpstreamSpec {
	spec := r.UpstreamConfig.spec()
	spec.Authentication = authenticationFromAPIKey(r.APIKey)
	return spec
}

func (c UpstreamConfig) spec() resource.UpstreamSpec {
	return resource.UpstreamSpec{
		DisplayName:       c.Name,
		Type:              c.Type,
		Protocol:          c.Protocol,
		TLS:               tlsFromRequest(c.TLS),
		Model:             modelSpecFromRequest(c.Model),
		LoadBalancePolicy: c.LoadBalancePolicy,
		HealthCheck:       c.HealthCheck,
		Endpoints:         endpointsFromRequest(c.Endpoints),
	}
}

func modelSpecFromRequest(request *ModelConfig) *resource.ModelSpec {
	if request == nil {
		return nil
	}
	models := make([]resource.ModelCatalogItem, 0, len(request.Models))
	for _, model := range request.Models {
		models = append(models, resource.ModelCatalogItem{
			Name:        model.Name,
			DisplayName: model.DisplayName,
			Enabled:     model.Enabled,
		})
	}
	return &resource.ModelSpec{
		Provider:    request.Provider,
		APIBasePath: request.APIBasePath,
		Models:      models,
	}
}

func authenticationFromAPIKey(apiKey *APIKeyConfig) *resource.UpstreamAuthentication {
	if apiKey == nil {
		return nil
	}
	return &resource.UpstreamAuthentication{
		APIKey: &resource.APIKeyAuthentication{Value: apiKey.Value},
	}
}

func tlsFromRequest(request *UpstreamTLS) *resource.UpstreamTLS {
	if request == nil {
		return nil
	}
	return &resource.UpstreamTLS{ServerName: request.ServerName}
}

func endpointsFromRequest(requests []UpstreamEndpoint) []resource.Endpoint {
	endpoints := make([]resource.Endpoint, 0, len(requests))
	for _, request := range requests {
		endpoints = append(endpoints, resource.Endpoint{
			Name:    request.ID,
			Address: request.Address,
			Port:    request.Port,
			Weight:  request.Weight,
			Enabled: request.Enabled,
		})
	}
	return endpoints
}
