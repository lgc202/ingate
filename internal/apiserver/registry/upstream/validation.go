package upstream

import (
	"net"
	"strconv"

	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
)

// validateUpstream 校验 HTTP 上游的连接配置和端点集合是否自洽。
func validateUpstream(upstream *resource.Upstream) field.ErrorList {
	specPath := field.NewPath("spec")
	spec := upstream.Spec
	errs := apiregistry.ValidateResourceID(upstream.Name, field.NewPath("metadata", "name"))

	errs = append(errs, apiregistry.ValidateDisplayName(
		spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	if !isSupportedLoadBalancingPolicy(spec.LoadBalancing) {
		errs = append(errs, field.NotSupported(
			specPath.Child("loadBalancing"),
			spec.LoadBalancing,
			[]string{
				string(resource.LoadBalancingRoundRobin),
				string(resource.LoadBalancingLeastRequest),
			},
		))
	}
	if spec.TLS != nil && !upstreamconfig.IsValidAddress(spec.TLS.ServerName) {
		errs = append(errs, field.Invalid(
			specPath.Child("tls", "serverName"),
			spec.TLS.ServerName,
			"serverName must be an IP address or DNS hostname",
		))
	}
	if spec.HealthCheck != nil {
		errs = append(errs, validateHealthCheck(
			spec.HealthCheck,
			specPath.Child("healthCheck"),
		)...)
	}
	if spec.Model != nil {
		errs = append(errs, validateModel(spec.Model, specPath.Child("model"))...)
	}
	errs = append(errs, validateEndpoints(spec.Endpoints, specPath.Child("endpoints"))...)
	return errs
}

func validateModel(model *resource.ModelUpstream, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	switch model.Protocol {
	case resource.ModelProtocolOpenAI, resource.ModelProtocolAnthropic:
	default:
		errs = append(errs, field.NotSupported(
			path.Child("protocol"),
			model.Protocol,
			[]string{
				string(resource.ModelProtocolOpenAI),
				string(resource.ModelProtocolAnthropic),
			},
		))
	}
	if !upstreamconfig.IsValidModelAPIKey(model.APIKey) {
		errs = append(errs, field.Invalid(
			path.Child("apiKey"),
			"<redacted>",
			"apiKey must not exceed 4096 bytes or contain surrounding whitespace",
		))
	}
	return errs
}

func validateEndpoints(endpoints []resource.Endpoint, path *field.Path) field.ErrorList {
	endpointCount := len(endpoints)
	if endpointCount == 0 {
		return field.ErrorList{field.Required(path, "at least one endpoint is required")}
	}

	var errs field.ErrorList
	if endpointCount > upstreamconfig.MaxEndpoints {
		errs = append(errs, field.TooMany(path, endpointCount, upstreamconfig.MaxEndpoints))
		endpoints = endpoints[:upstreamconfig.MaxEndpoints]
	}

	seenEndpointKeys := make(map[string]bool, len(endpoints))
	for i, endpoint := range endpoints {
		endpointPath := path.Index(i)
		if !upstreamconfig.IsValidAddress(endpoint.Address) {
			errs = append(errs, field.Invalid(
				endpointPath.Child("address"),
				endpoint.Address,
				"address must be an IP address or DNS hostname",
			))
		}
		if !upstreamconfig.IsValidEndpointPort(endpoint.Port) {
			errs = append(errs, field.Invalid(
				endpointPath.Child("port"),
				endpoint.Port,
				"port must be between 1 and 65535",
			))
		}
		if !upstreamconfig.IsValidEndpointWeight(endpoint.Weight) {
			errs = append(errs, field.Invalid(
				endpointPath.Child("weight"),
				endpoint.Weight,
				"weight must be between 1 and 1000",
			))
		}

		endpointKey := net.JoinHostPort(endpoint.Address, strconv.Itoa(endpoint.Port))
		if seenEndpointKeys[endpointKey] {
			errs = append(errs, field.Duplicate(endpointPath, endpointKey))
		} else {
			seenEndpointKeys[endpointKey] = true
		}
	}
	return errs
}

func validateHealthCheck(
	healthCheck *resource.UpstreamHealthCheck,
	path *field.Path,
) field.ErrorList {
	var errs field.ErrorList
	if !upstreamconfig.IsValidHealthCheckPath(healthCheck.Path) {
		errs = append(errs, field.Invalid(
			path.Child("path"),
			healthCheck.Path,
			"path must be an absolute request path without a query or fragment",
		))
	}
	if !upstreamconfig.IsValidHealthCheckInterval(healthCheck.IntervalSeconds) {
		errs = append(errs, field.Invalid(
			path.Child("intervalSeconds"),
			healthCheck.IntervalSeconds,
			"intervalSeconds must be between 1 and 300",
		))
	}
	if !upstreamconfig.IsValidHealthCheckTimeout(
		healthCheck.TimeoutSeconds,
		healthCheck.IntervalSeconds,
	) {
		errs = append(errs, field.Invalid(
			path.Child("timeoutSeconds"),
			healthCheck.TimeoutSeconds,
			"timeoutSeconds must be between 1 and 60 and less than intervalSeconds",
		))
	}
	return errs
}

func isSupportedLoadBalancingPolicy(policy resource.LoadBalancingPolicy) bool {
	switch policy {
	case resource.LoadBalancingRoundRobin, resource.LoadBalancingLeastRequest:
		return true
	default:
		return false
	}
}
