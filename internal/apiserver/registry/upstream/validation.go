package upstream

import (
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// validateUpstream 校验 HTTP 上游的连接配置和端点集合是否自洽
func validateUpstream(upstream *resource.Upstream) field.ErrorList {
	specPath := field.NewPath("spec")
	spec := upstream.Spec
	errs := field.ErrorList{}

	if spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	if !validLoadBalancing(spec.LoadBalancing) {
		errs = append(errs, field.NotSupported(specPath.Child("loadBalancing"), spec.LoadBalancing, []string{
			string(resource.LoadBalancingRoundRobin),
			string(resource.LoadBalancingLeastRequest),
		}))
	}
	if spec.TLS != nil && !validServerName(spec.TLS.ServerName) {
		errs = append(errs, field.Invalid(
			specPath.Child("tls", "serverName"),
			spec.TLS.ServerName,
			"serverName must be an IP address or DNS hostname",
		))
	}
	if spec.HealthCheck != nil {
		errs = append(errs, validateHealthCheck(spec.HealthCheck, specPath.Child("healthCheck"))...)
	}
	if spec.Model != nil {
		switch spec.Model.Protocol {
		case resource.ModelProtocolOpenAI, resource.ModelProtocolAnthropic:
		default:
			errs = append(errs, field.NotSupported(
				specPath.Child("model", "protocol"),
				spec.Model.Protocol,
				[]string{string(resource.ModelProtocolOpenAI), string(resource.ModelProtocolAnthropic)},
			))
		}
		if len(spec.Model.APIKey) > 4096 || strings.TrimSpace(spec.Model.APIKey) != spec.Model.APIKey {
			errs = append(errs, field.Invalid(
				specPath.Child("model", "apiKey"),
				"<redacted>",
				"apiKey must not exceed 4096 bytes or contain surrounding whitespace",
			))
		}
	}
	errs = append(errs, validateEndpoints(spec.Endpoints, specPath.Child("endpoints"))...)
	return errs
}

func validateEndpoints(endpoints []resource.Endpoint, endpointsPath *field.Path) field.ErrorList {
	if len(endpoints) == 0 {
		return field.ErrorList{field.Required(endpointsPath, "at least one endpoint is required")}
	}

	errs := field.ErrorList{}
	seen := make(map[string]struct{}, len(endpoints))
	for i, endpoint := range endpoints {
		endpointPath := endpointsPath.Index(i)
		if !validAddress(endpoint.Address) {
			errs = append(errs, field.Invalid(endpointPath.Child("address"), endpoint.Address, "address must be an IP address or DNS hostname"))
		}
		if endpoint.Port < 1 || endpoint.Port > 65535 {
			errs = append(errs, field.Invalid(endpointPath.Child("port"), endpoint.Port, "port must be between 1 and 65535"))
		}
		if endpoint.Weight < 1 || endpoint.Weight > 1000 {
			errs = append(errs, field.Invalid(endpointPath.Child("weight"), endpoint.Weight, "weight must be between 1 and 1000"))
		}

		key := net.JoinHostPort(endpoint.Address, strconv.Itoa(endpoint.Port))
		if _, exists := seen[key]; exists {
			errs = append(errs, field.Duplicate(endpointPath, key))
		} else {
			seen[key] = struct{}{}
		}
	}
	return errs
}

func validateHealthCheck(healthCheck *resource.UpstreamHealthCheck, healthCheckPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	if !validRequestPath(healthCheck.Path) {
		errs = append(errs, field.Invalid(healthCheckPath.Child("path"), healthCheck.Path, "path must be an absolute request path without a query or fragment"))
	}
	if healthCheck.IntervalSeconds < 1 || healthCheck.IntervalSeconds > 300 {
		errs = append(errs, field.Invalid(healthCheckPath.Child("intervalSeconds"), healthCheck.IntervalSeconds, "intervalSeconds must be between 1 and 300"))
	}
	if healthCheck.TimeoutSeconds < 1 || healthCheck.TimeoutSeconds > 60 || healthCheck.TimeoutSeconds >= healthCheck.IntervalSeconds {
		errs = append(errs, field.Invalid(healthCheckPath.Child("timeoutSeconds"), healthCheck.TimeoutSeconds, "timeoutSeconds must be between 1 and 60 and less than intervalSeconds"))
	}
	return errs
}

func validLoadBalancing(value resource.LoadBalancingPolicy) bool {
	switch value {
	case resource.LoadBalancingRoundRobin, resource.LoadBalancingLeastRequest:
		return true
	default:
		return false
	}
}

func validRequestPath(value string) bool {
	if !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "?#") {
		return false
	}
	_, err := url.ParseRequestURI(value)
	return err == nil
}

func validAddress(value string) bool {
	if value == "" {
		return false
	}
	if _, err := netip.ParseAddr(value); err == nil {
		return true
	}
	return validHostname(value)
}

func validServerName(value string) bool {
	return validAddress(value)
}

func validHostname(hostname string) bool {
	if hostname == "" || len(hostname) > 253 {
		return false
	}
	for label := range strings.SplitSeq(hostname, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if r < 'a' || r > 'z' {
				if r < '0' || r > '9' {
					if r != '-' {
						return false
					}
				}
			}
		}
	}
	return true
}
