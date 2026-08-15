package compiler

import (
	"cmp"
	"fmt"
	"slices"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Compile 将完整资源集合直接编译为可发布的 Envoy 配置
func Compile(resources Resources) Result {
	c := newCompilation(resources)
	c.indexResources(resources)
	clusters, endpoints, compiledUpstreams := c.buildUpstreams()
	listenerGroups, listenersByGateway := c.buildListenerGroups()
	routes, routeAttachments := c.buildRoutes(
		listenerGroups,
		listenersByGateway,
		compiledUpstreams,
	)
	filters, policyTargets := c.buildPolicyConfigs(routeAttachments)
	listeners := c.buildListeners(listenerGroups, filters)

	envoyConfig := EnvoyConfig{
		Listeners: listeners,
		Routes:    routes,
		Clusters:  clusters,
		Endpoints: endpoints,
	}
	sortEnvoyConfig(&envoyConfig)
	c.sortDiagnostics()

	resourceGenerations := resources.Generations()
	result := Result{
		ResourceGenerations: resourceGenerations,
		Diagnostics:         c.diagnostics,
	}
	if result.HasErrors() {
		return result
	}

	if err := envoyConfig.validate(); err != nil {
		c.addDiagnostic(SeverityError, "", "envoy", ReasonCompileFailed, fmt.Sprintf("validate Envoy configuration: %v", err))
		c.sortDiagnostics()
		return Result{ResourceGenerations: resourceGenerations, Diagnostics: c.diagnostics}
	}
	version, err := envoyConfig.version()
	if err != nil {
		c.addDiagnostic(SeverityError, "", "envoy", ReasonCompileFailed, fmt.Sprintf("compute Envoy config version: %v", err))
		c.sortDiagnostics()
		return Result{ResourceGenerations: resourceGenerations, Diagnostics: c.diagnostics}
	}

	result.Version = version
	result.Config = envoyConfig
	result.PolicyTargets = policyTargets
	return result
}

func (c *compilation) addDiagnostic(severity Severity, kind gatewayv1.Kind, id string, reason Reason, message string) {
	key := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", severity, kind, id, reason, message)
	if c.diagnosticSet[key] {
		return
	}
	c.diagnosticSet[key] = true
	c.diagnostics = append(c.diagnostics, Diagnostic{
		Severity: severity,
		Kind:     kind,
		ID:       id,
		Reason:   reason,
		Message:  message,
	})
}

func (c *compilation) sortDiagnostics() {
	slices.SortFunc(c.diagnostics, func(a, b Diagnostic) int {
		if a.Severity != b.Severity {
			if a.Severity == SeverityError {
				return -1
			}
			if b.Severity == SeverityError {
				return 1
			}
			return cmp.Compare(a.Severity, b.Severity)
		}
		if a.Kind != b.Kind {
			return cmp.Compare(a.Kind, b.Kind)
		}
		if a.ID != b.ID {
			return cmp.Compare(a.ID, b.ID)
		}
		if a.Reason != b.Reason {
			return cmp.Compare(a.Reason, b.Reason)
		}
		return cmp.Compare(a.Message, b.Message)
	})
}

func sortEnvoyConfig(config *EnvoyConfig) {
	slices.SortFunc(config.Listeners, func(a, b *listenerv3.Listener) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})
	slices.SortFunc(config.Routes, func(a, b *routev3.RouteConfiguration) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})
	slices.SortFunc(config.Clusters, func(a, b *clusterv3.Cluster) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})
	slices.SortFunc(config.Endpoints, func(a, b *endpointv3.ClusterLoadAssignment) int {
		return cmp.Compare(a.GetClusterName(), b.GetClusterName())
	})
}
