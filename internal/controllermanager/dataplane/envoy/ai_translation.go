package envoy

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// aiTranslation 保存最近有效的 AI 转换结果，由 Publisher 持锁读取并整体替换。
// 入口只依赖模型接口是否启用和 MCP 有序路径；目标沿用内容寻址名称，不另建业务标识。
// 目标名称覆盖绑定与网络的全部内容，端点排序也与网络转换一致，可以直接按名称复用。
// 每次候选仅保留当前使用的键，消息发布后只读，不保存调用方的可变集合。
type aiTranslation struct {
	resources      resourceSet
	entries        map[entryKey]aiEntryTranslation
	targets        map[string]aiTargetTranslation
	ingressSocket  string
	ingressCluster *clusterv3.Cluster
	egressSocket   string
	egressListener *listenerv3.Listener
	egressTargets  []string
	egressRoutes   *routev3.RouteConfiguration
}

type aiEntryTranslation struct {
	model  bool
	paths  []string
	routes []*routev3.Route
}

type aiTargetTranslation struct {
	cluster   *clusterv3.Cluster
	endpoints *endpointv3.ClusterLoadAssignment
	route     *routev3.Route
}

// compile 构建独立候选；完整结构校验不因命中缓存跳过，构建失败不修改上次结果。
func (t *aiTranslation) compile(config aidataplane.Config, ingressSocket, egressSocket string) (aiTranslation, error) {
	if err := aidataplane.Validate(config); err != nil {
		return aiTranslation{}, fmt.Errorf("validate AI configuration: %w", err)
	}

	next := aiTranslation{
		resources: resourceSet{ingressRoutes: make(map[entryKey][]*routev3.Route)},
		entries:   make(map[entryKey]aiEntryTranslation),
		targets:   make(map[string]aiTargetTranslation),
	}
	usedBackends := make(map[string]bool)
	usedNetworks := make(map[string]bool)
	for _, listener := range config.Listeners {
		for _, host := range listener.Hosts {
			key := entryKey{routeName: routeName(listener.Port), hostName: host.Name}
			entry, err := t.entries[key].compile(listener.Port, host)
			if err != nil {
				return aiTranslation{}, err
			}

			if len(entry.routes) != 0 {
				next.entries[key] = entry
				next.resources.ingressRoutes[key] = entry.routes
			}

			for _, route := range host.ModelRoutes {
				if *route.BackendName != "" {
					usedBackends[*route.BackendName] = true
				}
			}

			for _, route := range host.MCPRoutes {
				if *route.BackendName != "" {
					usedNetworks[*route.BackendName] = true
				}
			}
		}
	}

	if len(next.entries) != 0 {
		if !filepath.IsAbs(ingressSocket) || strings.ContainsRune(ingressSocket, 0) {
			return aiTranslation{}, errors.New("AI ingress socket must be an absolute path without NUL")
		}

		if filepath.Clean(ingressSocket) == filepath.Clean(egressSocket) {
			return aiTranslation{}, errors.New("AI ingress and egress sockets must differ")
		}

		cluster := t.ingressCluster
		if cluster == nil || t.ingressSocket != ingressSocket {
			var err error
			cluster, err = newAIIngressCluster(ingressSocket)
			if err != nil {
				return aiTranslation{}, err
			}
		}

		next.ingressSocket = ingressSocket
		next.ingressCluster = cluster
		next.resources.clusters = append(next.resources.clusters, cluster)
	}

	if len(usedBackends) == 0 && len(usedNetworks) == 0 {
		return next, nil
	}

	if !filepath.IsAbs(egressSocket) || strings.ContainsRune(egressSocket, 0) {
		return aiTranslation{}, errors.New("AI egress socket must be an absolute path without NUL")
	}

	networks := make(map[string]aidataplane.NetworkBackend, len(config.NetworkBackends))
	for _, network := range config.NetworkBackends {
		networks[network.Name] = network
	}

	for _, backend := range config.Backends {
		if !usedBackends[backend.Name] {
			continue
		}

		network := networks[backend.NetworkBackendName]
		target, err := aidataplane.ModelTargetName(backend, network)
		if err != nil {
			return aiTranslation{}, err
		}

		if err := next.addTarget(t, target, network); err != nil {
			return aiTranslation{}, err
		}
	}

	for _, name := range slices.Sorted(maps.Keys(usedNetworks)) {
		network := networks[name]
		target, err := aidataplane.MCPTargetName(network)
		if err != nil {
			return aiTranslation{}, err
		}

		if err := next.addTarget(t, target, network); err != nil {
			return aiTranslation{}, err
		}
	}

	routes := t.egressRoutes
	if routes == nil || !slices.Equal(t.egressTargets, next.egressTargets) {
		entries := make([]*routev3.Route, 0, len(next.egressTargets))
		for _, target := range next.egressTargets {
			entries = append(entries, next.targets[target].route)
		}

		var err error
		routes, err = newAIEgressRoutes(entries)
		if err != nil {
			return aiTranslation{}, err
		}
	}

	listener := t.egressListener
	if listener == nil || t.egressSocket != egressSocket {
		var err error
		listener, err = newAIListener(egressSocket)
		if err != nil {
			return aiTranslation{}, err
		}
	}

	next.egressSocket = egressSocket
	next.egressListener = listener
	next.egressRoutes = routes
	next.resources.routes = append(next.resources.routes, routes)
	next.resources.listeners = append(next.resources.listeners, listener)

	return next, nil
}

// addTarget 只修改本次候选，CDS/EDS 一起转换以保持 DNS 与 IP 形态切换一致。
func (t *aiTranslation) addTarget(previous *aiTranslation, target string, network aidataplane.NetworkBackend) error {
	translated := previous.targets[target]
	if translated.cluster == nil {
		cluster, endpoints, err := newAICluster(target, network)
		if err != nil {
			return err
		}

		translated.cluster = cluster
		translated.endpoints = endpoints
		translated.route = newAIEgressRoute(target, translated.cluster.Name)
	}

	t.targets[target] = translated
	t.egressTargets = append(t.egressTargets, target)
	t.resources.clusters = append(t.resources.clusters, translated.cluster)
	if translated.cluster.GetType() == clusterv3.Cluster_EDS {
		t.resources.endpoints = append(t.resources.endpoints, translated.endpoints)
	}

	return nil
}

// compile 忽略由 Go 解释的模型匹配、上游路径等字段；Envoy 只需要固定接口和 MCP 精确路径。
func (t aiEntryTranslation) compile(port int32, host aidataplane.Host) (aiEntryTranslation, error) {
	model := len(host.ModelRoutes) != 0
	var paths []string
	seen := make(map[string]bool, len(host.MCPRoutes))
	for _, route := range host.MCPRoutes {
		if model && (route.Path == aidataplane.ModelChatCompletionsPath || route.Path == aidataplane.ModelResponsesPath) {
			return aiEntryTranslation{}, fmt.Errorf("MCP route %q conflicts with model operations on host %q", route.Name, host.Name)
		}

		if !seen[route.Path] {
			seen[route.Path] = true
			paths = append(paths, route.Path)
		}
	}

	if t.model == model && slices.Equal(t.paths, paths) {
		return t, nil
	}

	next := aiEntryTranslation{model: model, paths: paths}
	if model {
		next.routes = append(next.routes,
			newAIIngressRoute(port, host.Name, aidataplane.ModelChatCompletionsPath, "model"),
			newAIIngressRoute(port, host.Name, aidataplane.ModelResponsesPath, "model"),
		)
	}

	for _, path := range paths {
		next.routes = append(next.routes, newAIIngressRoute(port, host.Name, path, "mcp"))
	}

	return next, nil
}
