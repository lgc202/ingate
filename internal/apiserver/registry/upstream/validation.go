package upstream

import (
	"net/netip"
	"net/url"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/internal/pkg/httpheader"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// validateUpstream 校验一个网络目标的协议、认证、健康检查和端点配置是否自洽
func validateUpstream(upstream *resource.Upstream) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := field.ErrorList{}

	if len(upstream.Spec.Endpoints) == 0 {
		errs = append(errs, field.Required(specPath.Child("endpoints"), "at least one endpoint is required"))
	}
	if upstream.Spec.Type == "" {
		errs = append(errs, field.Required(specPath.Child("type"), "type is required"))
	} else if !validUpstreamType(upstream.Spec.Type) {
		errs = append(errs, field.NotSupported(specPath.Child("type"), upstream.Spec.Type, []string{
			string(resource.UpstreamTypeApplication),
			string(resource.UpstreamTypeModel),
			string(resource.UpstreamTypeAgent),
			string(resource.UpstreamTypeMCP),
		}))
	}
	if upstream.Spec.Protocol == "" {
		errs = append(errs, field.Required(specPath.Child("protocol"), "protocol is required"))
	} else if !upstream.Spec.Protocol.IsSupported() {
		errs = append(errs, field.NotSupported(specPath.Child("protocol"), upstream.Spec.Protocol, []string{
			string(resource.UpstreamProtocolHTTP),
			string(resource.UpstreamProtocolOpenAI),
			string(resource.UpstreamProtocolAnthropic),
			string(resource.UpstreamProtocolGemini),
		}))
	}
	if upstream.Spec.Type == resource.UpstreamTypeModel {
		if upstream.Spec.Model == nil {
			errs = append(errs, field.Required(specPath.Child("model"), "model is required for model upstreams"))
		} else {
			errs = append(errs, validateModelSpec(upstream.Spec.Model, upstream.Spec.Protocol, specPath.Child("model"), specPath.Child("protocol"))...)
		}
	} else if upstream.Spec.Type != "" {
		if upstream.Spec.Model != nil {
			errs = append(errs, field.Forbidden(specPath.Child("model"), "model is only supported by model upstreams"))
		}
		if upstream.Spec.Protocol != "" && upstream.Spec.Protocol != resource.UpstreamProtocolHTTP {
			errs = append(errs, field.Invalid(specPath.Child("protocol"), upstream.Spec.Protocol, "non-model upstreams must use the HTTP protocol"))
		}
	}
	if upstream.Spec.Authentication != nil {
		authenticationPath := specPath.Child("authentication")
		if upstream.Spec.Authentication.APIKey == nil {
			errs = append(errs, field.Required(authenticationPath.Child("apiKey"), "apiKey is required when authentication is configured"))
		} else if upstream.Spec.Authentication.APIKey.Value == "" || !httpheader.ValidValue(upstream.Spec.Authentication.APIKey.Value) {
			errs = append(errs, field.Invalid(authenticationPath.Child("apiKey", "value"), "<redacted>", "value must be safe for use in an HTTP header"))
		}
		if upstream.Spec.Type != resource.UpstreamTypeModel {
			errs = append(errs, field.Forbidden(authenticationPath, "authentication is only supported by model upstreams"))
		}
		if upstream.Spec.TLS == nil {
			errs = append(errs, field.Required(specPath.Child("tls"), "tls is required when authentication is configured"))
		}
	}
	if upstream.Spec.TLS != nil {
		tlsPath := specPath.Child("tls")
		if !validTLSServerName(upstream.Spec.TLS.ServerName) {
			errs = append(errs, field.Invalid(tlsPath.Child("serverName"), upstream.Spec.TLS.ServerName, "serverName must be an IP address or DNS hostname"))
		}
	}
	if upstream.Spec.LoadBalancePolicy != "" && !validLoadBalancePolicy(upstream.Spec.LoadBalancePolicy) {
		errs = append(errs, field.NotSupported(specPath.Child("loadBalancePolicy"), upstream.Spec.LoadBalancePolicy, []string{
			string(resource.UpstreamLoadBalancePolicyRoundRobin),
			string(resource.UpstreamLoadBalancePolicyLeastRequest),
			string(resource.UpstreamLoadBalancePolicyRandom),
		}))
	}
	if upstream.Spec.HealthCheck != nil {
		healthCheckPath := specPath.Child("healthCheck")
		if upstream.Spec.HealthCheck.Enabled {
			if upstream.Spec.HealthCheck.Path == "" {
				errs = append(errs, field.Required(healthCheckPath.Child("path"), "path is required when health check is enabled"))
			}
			if upstream.Spec.HealthCheck.IntervalSeconds < 1 || upstream.Spec.HealthCheck.IntervalSeconds > 300 {
				errs = append(errs, field.Invalid(healthCheckPath.Child("intervalSeconds"), upstream.Spec.HealthCheck.IntervalSeconds, "intervalSeconds must be between 1 and 300"))
			}
			if upstream.Spec.HealthCheck.TimeoutSeconds < 1 || upstream.Spec.HealthCheck.TimeoutSeconds > 60 || upstream.Spec.HealthCheck.TimeoutSeconds >= upstream.Spec.HealthCheck.IntervalSeconds {
				errs = append(errs, field.Invalid(healthCheckPath.Child("timeoutSeconds"), upstream.Spec.HealthCheck.TimeoutSeconds, "timeoutSeconds must be between 1 and 60 and less than intervalSeconds"))
			}
		}
	}

	enabledEndpoints := 0
	endpointNames := map[string]struct{}{}
	for i, endpoint := range upstream.Spec.Endpoints {
		endpointPath := specPath.Child("endpoints").Index(i)
		if endpoint.Name != "" {
			if _, ok := endpointNames[endpoint.Name]; ok {
				errs = append(errs, field.Duplicate(endpointPath.Child("name"), endpoint.Name))
			}
			endpointNames[endpoint.Name] = struct{}{}
		}
		if endpoint.Address == "" {
			errs = append(errs, field.Required(endpointPath.Child("address"), "address is required"))
		}
		if endpoint.Port < 1 || endpoint.Port > 65535 {
			errs = append(errs, field.Invalid(endpointPath.Child("port"), endpoint.Port, "port must be between 1 and 65535"))
		}
		if endpoint.Weight < 1 || endpoint.Weight > 100 {
			errs = append(errs, field.Invalid(endpointPath.Child("weight"), endpoint.Weight, "weight must be between 1 and 100"))
		}
		if endpoint.Enabled {
			enabledEndpoints++
		}
	}
	if len(upstream.Spec.Endpoints) > 0 && enabledEndpoints == 0 {
		errs = append(errs, field.Invalid(specPath.Child("endpoints"), upstream.Spec.Endpoints, "at least one endpoint must be enabled"))
	}

	return errs
}

func validUpstreamType(value resource.UpstreamType) bool {
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

// validateModelSpec 保证模型厂商与上游协议一致，并校验用户维护的模型目录
func validateModelSpec(
	model *resource.ModelSpec,
	protocol resource.UpstreamProtocol,
	modelPath *field.Path,
	protocolPath *field.Path,
) field.ErrorList {
	errs := field.ErrorList{}
	providerProtocol, validProvider := model.Provider.Protocol()
	if !validProvider {
		errs = append(errs, field.NotSupported(modelPath.Child("provider"), model.Provider, []string{
			string(resource.ModelProviderOpenAI),
			string(resource.ModelProviderDeepSeek),
			string(resource.ModelProviderQwen),
			string(resource.ModelProviderAnthropic),
			string(resource.ModelProviderGemini),
			string(resource.ModelProviderCustom),
		}))
	} else if protocol != providerProtocol {
		errs = append(errs, field.Invalid(protocolPath, protocol, "protocol does not match model provider"))
	}
	if !validAPIBasePath(model.APIBasePath) {
		errs = append(errs, field.Invalid(modelPath.Child("apiBasePath"), model.APIBasePath, "apiBasePath must be a normalized absolute path without query, fragment, or trailing slash"))
	}
	if len(model.Models) == 0 {
		return append(errs, field.Required(modelPath.Child("models"), "at least one model is required"))
	}

	enabledModels := 0
	modelNames := make(map[string]struct{}, len(model.Models))
	for i, item := range model.Models {
		itemPath := modelPath.Child("models").Index(i)
		if item.Name == "" {
			errs = append(errs, field.Required(itemPath.Child("name"), "name is required"))
		} else if strings.TrimSpace(item.Name) != item.Name {
			errs = append(errs, field.Invalid(itemPath.Child("name"), item.Name, "name must not contain leading or trailing whitespace"))
		} else if _, exists := modelNames[item.Name]; exists {
			errs = append(errs, field.Duplicate(itemPath.Child("name"), item.Name))
		} else {
			modelNames[item.Name] = struct{}{}
		}
		if item.DisplayName == "" {
			errs = append(errs, field.Required(itemPath.Child("displayName"), "displayName is required"))
		} else if strings.TrimSpace(item.DisplayName) != item.DisplayName {
			errs = append(errs, field.Invalid(itemPath.Child("displayName"), item.DisplayName, "displayName must not contain leading or trailing whitespace"))
		}
		if item.Enabled {
			enabledModels++
		}
	}
	if enabledModels == 0 {
		errs = append(errs, field.Invalid(modelPath.Child("models"), model.Models, "at least one model must be enabled"))
	}
	return errs
}

func validAPIBasePath(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || !strings.HasPrefix(value, "/") {
		return false
	}
	if value != "/" && strings.HasSuffix(value, "/") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != value {
		return false
	}
	return path.Clean(value) == value
}

func validTLSServerName(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	if _, err := netip.ParseAddr(value); err == nil {
		return true
	}
	return validHostname(strings.ToLower(value))
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
