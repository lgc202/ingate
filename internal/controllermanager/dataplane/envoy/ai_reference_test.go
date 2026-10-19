package envoy

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// buildAIResourcesReference 保留无缓存的完整转换作为测试基准，不进入生产发布路径。
// 出口 Header 精确匹配静态 Cluster，不使用允许请求选择任意 Cluster 的 cluster_header。
func buildAIResourcesReference(config aidataplane.Config, ingressSocket, egressSocket string) (resourceSet, error) {
	if err := aidataplane.Validate(config); err != nil {
		return resourceSet{}, fmt.Errorf("validate AI configuration: %w", err)
	}

	resources := resourceSet{ingressRoutes: make(map[entryKey][]*routev3.Route)}
	usedBackends := make(map[string]bool)
	usedNetworks := make(map[string]bool)
	for _, listener := range config.Listeners {
		for _, host := range listener.Hosts {
			key := entryKey{routeName: routeName(listener.Port), hostName: host.Name}
			if len(host.ModelRoutes) != 0 {
				resources.ingressRoutes[key] = append(resources.ingressRoutes[key],
					newAIIngressRoute(listener.Port, host.Name, aidataplane.ModelChatCompletionsPath, "model"),
					newAIIngressRoute(listener.Port, host.Name, aidataplane.ModelResponsesPath, "model"),
				)
			}

			for _, route := range host.ModelRoutes {
				if *route.BackendName != "" {
					usedBackends[*route.BackendName] = true
				}
			}

			// 同一 Host 内相同精确路径只保留编译顺序的首个候选，避免重复 Envoy 路由。
			entryPaths := make(map[string]bool, len(host.MCPRoutes))
			for _, route := range host.MCPRoutes {
				if len(host.ModelRoutes) != 0 && (route.Path == aidataplane.ModelChatCompletionsPath || route.Path == aidataplane.ModelResponsesPath) {
					return resourceSet{}, fmt.Errorf("MCP route %q conflicts with model operations on host %q", route.Name, host.Name)
				}

				if !entryPaths[route.Path] {
					entryPaths[route.Path] = true
					resources.ingressRoutes[key] = append(resources.ingressRoutes[key],
						newAIIngressRoute(listener.Port, host.Name, route.Path, "mcp"),
					)
				}

				if *route.BackendName != "" {
					usedNetworks[*route.BackendName] = true
				}
			}
		}
	}

	if len(resources.ingressRoutes) != 0 {
		if !filepath.IsAbs(ingressSocket) || strings.ContainsRune(ingressSocket, 0) {
			return resourceSet{}, errors.New("AI ingress socket must be an absolute path without NUL")
		}

		if filepath.Clean(ingressSocket) == filepath.Clean(egressSocket) {
			return resourceSet{}, errors.New("AI ingress and egress sockets must differ")
		}

		cluster, err := newAIIngressCluster(ingressSocket)
		if err != nil {
			return resourceSet{}, err
		}

		resources.clusters = append(resources.clusters, cluster)
	}

	if len(usedBackends) == 0 && len(usedNetworks) == 0 {
		return resources, nil
	}

	if !filepath.IsAbs(egressSocket) || strings.ContainsRune(egressSocket, 0) {
		return resourceSet{}, errors.New("AI egress socket must be an absolute path without NUL")
	}

	networks := make(map[string]aidataplane.NetworkBackend, len(config.NetworkBackends))
	for _, network := range config.NetworkBackends {
		networks[network.Name] = network
	}

	host := &routev3.VirtualHost{Name: aiEgressName, Domains: []string{"*"}}
	for _, backend := range config.Backends {
		if !usedBackends[backend.Name] {
			continue
		}

		network := networks[backend.NetworkBackendName]
		target, err := aidataplane.ModelTargetName(backend, network)
		if err != nil {
			return resourceSet{}, err
		}

		cluster, assignment, err := newAICluster(target, network)
		if err != nil {
			return resourceSet{}, err
		}

		resources.clusters = append(resources.clusters, cluster)
		if cluster.GetType() == clusterv3.Cluster_EDS {
			resources.endpoints = append(resources.endpoints, assignment)
		}

		host.Routes = append(host.Routes, newAIEgressRoute(target, cluster.Name))
	}

	// 同一网络被多个 MCP 入口复用时只建一个出口；按名称排序保持发布内容确定。
	for _, name := range slices.Sorted(maps.Keys(usedNetworks)) {
		network := networks[name]
		target, err := aidataplane.MCPTargetName(network)
		if err != nil {
			return resourceSet{}, err
		}

		cluster, assignment, err := newAICluster(target, network)
		if err != nil {
			return resourceSet{}, err
		}

		resources.clusters = append(resources.clusters, cluster)
		if cluster.GetType() == clusterv3.Cluster_EDS {
			resources.endpoints = append(resources.endpoints, assignment)
		}

		host.Routes = append(host.Routes, newAIEgressRoute(target, cluster.Name))
	}

	// 未知、缺失或已撤销目标明确拒绝，不能命中普通 HTTP 或其他 AI 后端。
	host.Routes = append(host.Routes, &routev3.Route{
		Name:   aiEgressName + "/unavailable",
		Match:  &routev3.RouteMatch{PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: "/"}},
		Action: &routev3.Route_DirectResponse{DirectResponse: &routev3.DirectResponseAction{Status: 503}},
	})
	routes := &routev3.RouteConfiguration{Name: aiEgressName, VirtualHosts: []*routev3.VirtualHost{host}}
	if err := routes.ValidateAll(); err != nil {
		return resourceSet{}, fmt.Errorf("validate AI egress routes: %w", err)
	}

	listener, err := newAIListener(egressSocket)
	if err != nil {
		return resourceSet{}, err
	}

	resources.routes = append(resources.routes, routes)
	resources.listeners = append(resources.listeners, listener)

	return resources, nil
}
