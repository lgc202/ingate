package validation

import (
	"fmt"
	"net"
	"strings"

	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

var (
	validGatewayProtocols = map[string]struct{}{"HTTP": {}, "HTTPS": {}}
	validRoutePathTypes   = map[string]struct{}{"Exact": {}, "PathPrefix": {}}
	validRouteFilterTypes = map[string]struct{}{
		"URLRewrite":             {},
		"RequestHeaderModifier":  {},
		"ResponseHeaderModifier": {},
	}
	validPathRewriteTypes = map[string]struct{}{"ReplacePrefixMatch": {}}
	validBackendTypes     = map[string]struct{}{"Static": {}, "DNS": {}}
	validBackendProtocols = map[string]struct{}{"HTTP": {}, "HTTPS": {}, "gRPC": {}}
	validLoadBalances     = map[string]struct{}{"": {}, "RoundRobin": {}, "LeastRequest": {}}
	validSecretTypes      = map[string]struct{}{"Opaque": {}, "kubernetes.io/tls": {}}
	validHTTPMethods      = map[string]struct{}{
		"": {}, "GET": {}, "POST": {}, "PUT": {}, "PATCH": {}, "DELETE": {}, "HEAD": {}, "OPTIONS": {},
	}
)

func ValidateGateway(gateway *gatewayv1alpha1.Gateway) field.ErrorList {
	var allErrs field.ErrorList

	if len(gateway.Spec.Listeners) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "listeners"), "at least one listener is required"))
	}

	seenNames := map[string]struct{}{}
	seenPortProtocols := map[string]struct{}{}
	for i, listener := range gateway.Spec.Listeners {
		fldPath := field.NewPath("spec", "listeners").Index(i)

		if listener.Name == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("name"), "listener name is required"))
		} else {
			if _, exists := seenNames[listener.Name]; exists {
				allErrs = append(allErrs, field.Duplicate(fldPath.Child("name"), listener.Name))
			}
			seenNames[listener.Name] = struct{}{}
		}

		if _, ok := validGatewayProtocols[listener.Protocol]; !ok {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("protocol"), listener.Protocol, []string{"HTTP", "HTTPS"}))
		}

		if listener.Port < 1 || listener.Port > 65535 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("port"), listener.Port, "must be a valid TCP port"))
		}

		if listener.Protocol != "" && listener.Port >= 1 && listener.Port <= 65535 {
			key := fmt.Sprintf("%s:%d", listener.Protocol, listener.Port)
			if _, exists := seenPortProtocols[key]; exists {
				allErrs = append(allErrs, field.Duplicate(fldPath.Child("port"), listener.Port))
			}
			seenPortProtocols[key] = struct{}{}
		}

		seenHostnames := map[string]struct{}{}
		for j, hostname := range gatewayListenerHostnames(listener) {
			hostnamePath := fldPath.Child("hostnames").Index(j)
			if hostname == "" {
				allErrs = append(allErrs, field.Required(hostnamePath, "hostname is required"))
				continue
			}
			if _, exists := seenHostnames[hostname]; exists {
				allErrs = append(allErrs, field.Duplicate(hostnamePath, hostname))
				continue
			}
			seenHostnames[hostname] = struct{}{}
			if len(utilvalidation.IsDNS1123Subdomain(hostname)) != 0 {
				allErrs = append(allErrs, field.Invalid(hostnamePath, hostname, "must be a valid DNS subdomain"))
			}
		}

		switch listener.Protocol {
		case "HTTP":
			if listener.TLS != nil {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("tls"), "HTTP listeners must not define tls"))
			}
		case "HTTPS":
			if listener.TLS == nil {
				allErrs = append(allErrs, field.Required(fldPath.Child("tls"), "HTTPS listeners must define tls"))
				continue
			}
			if listener.TLS.Mode != "Terminate" {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("tls", "mode"), listener.TLS.Mode, []string{"Terminate"}))
			}
			if listener.TLS.CertificateRef == nil || listener.TLS.CertificateRef.Name == "" {
				allErrs = append(allErrs, field.Required(fldPath.Child("tls", "certificateRef", "name"), "certificateRef.name is required"))
			}
		}
	}

	if gateway.Spec.AllowedRoutes != nil {
		for i, kind := range gateway.Spec.AllowedRoutes.Kinds {
			if kind != "Route" {
				allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "allowedRoutes", "kinds").Index(i), kind, []string{"Route"}))
			}
		}
	}

	return allErrs
}

func gatewayListenerHostnames(listener gatewayv1alpha1.GatewayListener) []string {
	if len(listener.Hostnames) > 0 {
		return listener.Hostnames
	}
	if listener.Hostname != "" {
		return []string{listener.Hostname}
	}
	return nil
}

func ValidateGatewayUpdate(update, old *gatewayv1alpha1.Gateway) field.ErrorList {
	return ValidateGateway(update)
}

func ValidateCertificate(certificate *gatewayv1alpha1.Certificate) field.ErrorList {
	var allErrs field.ErrorList

	if certificate.Spec.SecretRef.Name == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "secretRef", "name"), "secretRef.name is required"))
	}

	seenDomains := map[string]struct{}{}
	for i, domain := range certificate.Spec.Domains {
		fldPath := field.NewPath("spec", "domains").Index(i)
		if domain == "" {
			allErrs = append(allErrs, field.Required(fldPath, "domain is required"))
			continue
		}
		if _, exists := seenDomains[domain]; exists {
			allErrs = append(allErrs, field.Duplicate(fldPath, domain))
		}
		seenDomains[domain] = struct{}{}

		if !isValidCertificateDomain(domain) {
			allErrs = append(allErrs, field.Invalid(fldPath, domain, "must be a valid DNS subdomain or wildcard DNS subdomain"))
		}
	}

	return allErrs
}

func ValidateCertificateUpdate(update, old *gatewayv1alpha1.Certificate) field.ErrorList {
	return ValidateCertificate(update)
}

func ValidateSecret(secret *gatewayv1alpha1.Secret) field.ErrorList {
	var allErrs field.ErrorList

	if _, ok := validSecretTypes[secret.Spec.Type]; !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "type"), secret.Spec.Type, []string{"Opaque", "kubernetes.io/tls"}))
	}
	if len(secret.Spec.StringData) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "stringData"), "at least one key is required"))
	}
	for key, value := range secret.Spec.StringData {
		if key == "" {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "stringData").Key(key), "key name is required"))
		}
		if value == "" {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "stringData").Key(key), "secret value is required"))
		}
	}

	if secret.Spec.Type == "kubernetes.io/tls" {
		if secret.Spec.StringData["tls.crt"] == "" {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "stringData").Key("tls.crt"), "tls.crt is required for TLS secrets"))
		}
		if secret.Spec.StringData["tls.key"] == "" {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "stringData").Key("tls.key"), "tls.key is required for TLS secrets"))
		}
	}

	return allErrs
}

func ValidateSecretUpdate(update, old *gatewayv1alpha1.Secret) field.ErrorList {
	return ValidateSecret(update)
}

func ValidateRoute(route *gatewayv1alpha1.Route) field.ErrorList {
	var allErrs field.ErrorList

	if len(route.Spec.ParentRefs) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "parentRefs"), "at least one parentRef is required"))
	}
	for i, parentRef := range route.Spec.ParentRefs {
		if parentRef.Name == "" {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "parentRefs").Index(i).Child("name"), "parentRef name is required"))
		}
	}

	if len(route.Spec.Rules) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "rules"), "at least one rule is required"))
	}
	for i, rule := range route.Spec.Rules {
		rulePath := field.NewPath("spec", "rules").Index(i)
		if len(rule.Matches) == 0 {
			allErrs = append(allErrs, field.Required(rulePath.Child("matches"), "at least one match is required"))
		}
		for j, match := range rule.Matches {
			matchPath := rulePath.Child("matches").Index(j)
			if match.Path != nil {
				if _, ok := validRoutePathTypes[match.Path.Type]; !ok {
					allErrs = append(allErrs, field.NotSupported(matchPath.Child("path", "type"), match.Path.Type, []string{"Exact", "PathPrefix"}))
				}
				if match.Path.Value == "" {
					allErrs = append(allErrs, field.Required(matchPath.Child("path", "value"), "path value is required"))
				}
			}
			method := strings.ToUpper(match.Method)
			if _, ok := validHTTPMethods[method]; !ok {
				allErrs = append(allErrs, field.NotSupported(matchPath.Child("method"), match.Method, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}))
			}
			for k, header := range match.Headers {
				headerPath := matchPath.Child("headers").Index(k)
				if header.Name == "" {
					allErrs = append(allErrs, field.Required(headerPath.Child("name"), "header name is required"))
				}
				if header.Value == "" {
					allErrs = append(allErrs, field.Required(headerPath.Child("value"), "header value is required"))
				}
			}
		}

		if len(rule.BackendRefs) == 0 {
			allErrs = append(allErrs, field.Required(rulePath.Child("backendRefs"), "at least one backendRef is required"))
		}
		for j, backendRef := range rule.BackendRefs {
			refPath := rulePath.Child("backendRefs").Index(j)
			if backendRef.Name == "" {
				allErrs = append(allErrs, field.Required(refPath.Child("name"), "backendRef name is required"))
			}
			if backendRef.Port < 0 || backendRef.Port > 65535 {
				allErrs = append(allErrs, field.Invalid(refPath.Child("port"), backendRef.Port, "must be a valid TCP port"))
			}
			if backendRef.Weight <= 0 {
				allErrs = append(allErrs, field.Invalid(refPath.Child("weight"), backendRef.Weight, "must be a positive integer"))
			}
		}

		for j, filter := range rule.Filters {
			filterPath := rulePath.Child("filters").Index(j)
			if _, ok := validRouteFilterTypes[filter.Type]; !ok {
				allErrs = append(allErrs, field.NotSupported(filterPath.Child("type"), filter.Type, []string{"URLRewrite", "RequestHeaderModifier", "ResponseHeaderModifier"}))
			}
			switch filter.Type {
			case "URLRewrite":
				if filter.URLRewrite == nil {
					allErrs = append(allErrs, field.Required(filterPath.Child("urlRewrite"), "urlRewrite is required for URLRewrite filters"))
					continue
				}
				if filter.URLRewrite.Path != nil {
					path := filter.URLRewrite.Path
					if _, ok := validPathRewriteTypes[path.Type]; !ok {
						allErrs = append(allErrs, field.NotSupported(filterPath.Child("urlRewrite", "path", "type"), path.Type, []string{"ReplacePrefixMatch"}))
					}
					if path.ReplacePrefixMatch == "" {
						allErrs = append(allErrs, field.Required(filterPath.Child("urlRewrite", "path", "replacePrefixMatch"), "replacePrefixMatch is required"))
					} else if !strings.HasPrefix(path.ReplacePrefixMatch, "/") {
						allErrs = append(allErrs, field.Invalid(filterPath.Child("urlRewrite", "path", "replacePrefixMatch"), path.ReplacePrefixMatch, "must start with /"))
					}
				}
			case "RequestHeaderModifier":
				allErrs = append(allErrs, validateHeaderFilter(filter.RequestHeaderModifier, filterPath.Child("requestHeaderModifier"))...)
			case "ResponseHeaderModifier":
				allErrs = append(allErrs, validateHeaderFilter(filter.ResponseHeaderModifier, filterPath.Child("responseHeaderModifier"))...)
			}
		}
	}

	return allErrs
}

func ValidateRouteUpdate(update, old *gatewayv1alpha1.Route) field.ErrorList {
	return ValidateRoute(update)
}

func validateHeaderFilter(filter *gatewayv1alpha1.HTTPHeaderFilter, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if filter == nil {
		return append(allErrs, field.Required(fldPath, "header modifier is required"))
	}
	if len(filter.Set) == 0 && len(filter.Add) == 0 && len(filter.Remove) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "at least one header action is required"))
	}
	for i, header := range filter.Set {
		headerPath := fldPath.Child("set").Index(i)
		if header.Name == "" {
			allErrs = append(allErrs, field.Required(headerPath.Child("name"), "header name is required"))
		}
		if header.Value == "" {
			allErrs = append(allErrs, field.Required(headerPath.Child("value"), "header value is required"))
		}
	}
	for i, header := range filter.Add {
		headerPath := fldPath.Child("add").Index(i)
		if header.Name == "" {
			allErrs = append(allErrs, field.Required(headerPath.Child("name"), "header name is required"))
		}
		if header.Value == "" {
			allErrs = append(allErrs, field.Required(headerPath.Child("value"), "header value is required"))
		}
	}
	for i, headerName := range filter.Remove {
		if headerName == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("remove").Index(i), "header name is required"))
		}
	}
	return allErrs
}

func ValidateBackend(backend *gatewayv1alpha1.Backend) field.ErrorList {
	return validateBackend(backend, nil, false)
}

func ValidateBackendUpdate(update, old *gatewayv1alpha1.Backend) field.ErrorList {
	return validateBackend(update, old, true)
}

func validateBackend(backend *gatewayv1alpha1.Backend, old *gatewayv1alpha1.Backend, requireProtocol bool) field.ErrorList {
	var allErrs field.ErrorList

	if _, ok := validBackendTypes[backend.Spec.Type]; !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "type"), backend.Spec.Type, []string{"Static", "DNS"}))
	}

	if backend.Spec.Protocol == "" {
		if requireProtocol && (old == nil || old.Spec.Protocol != "") {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "protocol"), "backend protocol is required"))
		}
	} else if _, ok := validBackendProtocols[backend.Spec.Protocol]; !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "protocol"), backend.Spec.Protocol, []string{"HTTP", "HTTPS", "gRPC"}))
	}

	if backend.Spec.DefaultPort < 1 || backend.Spec.DefaultPort > 65535 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "defaultPort"), backend.Spec.DefaultPort, "must be a valid TCP port"))
	}

	if backend.Spec.LoadBalance != nil {
		if _, ok := validLoadBalances[backend.Spec.LoadBalance.Policy]; !ok {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "loadBalance", "policy"), backend.Spec.LoadBalance.Policy, []string{"RoundRobin", "LeastRequest"}))
		}
	}

	if backend.Spec.Static != nil && backend.Spec.DNS != nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec"), "static and dns backends cannot be configured together"))
	}

	switch backend.Spec.Type {
	case "Static":
		if backend.Spec.Static == nil || len(backend.Spec.Static.Endpoints) == 0 {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "static", "endpoints"), "static backend requires endpoints"))
		}
	case "DNS":
		if backend.Spec.DNS == nil || backend.Spec.DNS.Host == "" {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "dns", "host"), "dns backend requires host"))
		}
		if backend.Spec.DNS != nil && backend.Spec.DNS.Port != 0 && (backend.Spec.DNS.Port < 1 || backend.Spec.DNS.Port > 65535) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "dns", "port"), backend.Spec.DNS.Port, "must be a valid TCP port"))
		}
	}

	if backend.Spec.Static != nil {
		for i, endpoint := range backend.Spec.Static.Endpoints {
			allErrs = append(allErrs, validateEndpoint(endpoint, field.NewPath("spec", "static", "endpoints").Index(i))...)
		}
	}

	return allErrs
}

func validateEndpoint(endpoint gatewayv1alpha1.BackendEndpoint, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if endpoint.Address == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("address"), "endpoint address is required"))
	} else if net.ParseIP(endpoint.Address) == nil && len(utilvalidation.IsDNS1123Subdomain(endpoint.Address)) != 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("address"), endpoint.Address, "must be a valid IP address or DNS subdomain"))
	}

	if endpoint.Port < 1 || endpoint.Port > 65535 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("port"), endpoint.Port, "must be a valid TCP port"))
	}

	if endpoint.Weight < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("weight"), endpoint.Weight, "must be zero or a positive integer"))
	}

	return allErrs
}

func isValidCertificateDomain(domain string) bool {
	if strings.HasPrefix(domain, "*.") {
		return len(utilvalidation.IsDNS1123Subdomain(strings.TrimPrefix(domain, "*."))) == 0
	}
	return len(utilvalidation.IsDNS1123Subdomain(domain)) == 0
}
