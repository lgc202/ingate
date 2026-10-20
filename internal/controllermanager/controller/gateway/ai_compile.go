package gateway

import (
	"cmp"
	"maps"
	"path"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// aiCompilation 只保存一次纯编译的索引和结果，不作为协调器缓存。
type aiCompilation struct {
	gateways         []*gatewayv1alpha1.Gateway
	aiBackends       map[client.ObjectKey]*modelBackendBinding
	backends         map[client.ObjectKey]*networkBackend
	modelRoutes      map[listenerRef][]aidataplane.ModelRoute
	mcpRoutes        map[listenerRef][]aidataplane.MCPRoute
	protocolBackends map[string]aidataplane.Backend
	networkBackends  map[string]bool
}

func resolveAI(snapshot resources) (*aiCompilation, error) {
	var accepted resolution
	accepted.selectGateways(snapshot.gateways.Items)

	c := aiCompilation{
		gateways:         accepted.gateways,
		aiBackends:       make(map[client.ObjectKey]*modelBackendBinding),
		backends:         make(map[client.ObjectKey]*networkBackend),
		modelRoutes:      make(map[listenerRef][]aidataplane.ModelRoute),
		mcpRoutes:        make(map[listenerRef][]aidataplane.MCPRoute),
		protocolBackends: make(map[string]aidataplane.Backend),
		networkBackends:  make(map[string]bool),
	}
	for _, binding := range snapshot.aiBackends {
		if !binding.backend.deleting {
			c.aiBackends[binding.backend.key] = binding
		}
	}

	for _, backend := range snapshot.backends {
		if !backend.deleting {
			c.backends[backend.key] = backend
		}
	}

	if err := c.compileModelRoutes(snapshot.modelRoutes); err != nil {
		return nil, err
	}

	if err := c.compileMCPRoutes(snapshot.mcpRoutes); err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *aiCompilation) buildConfig() aidataplane.Config {
	config := aidataplane.Config{
		FormatVersion:   aidataplane.FormatVersionV1,
		Listeners:       []aidataplane.Listener{},
		Backends:        []aidataplane.Backend{},
		NetworkBackends: []aidataplane.NetworkBackend{},
	}
	listeners := make(map[int32]*aidataplane.Listener)
	for _, gateway := range c.gateways {
		key := client.ObjectKeyFromObject(gateway)
		for _, listener := range gateway.Spec.Listeners {
			ref := listenerRef{gatewayKey: key, listenerName: listener.Name}
			models := c.modelRoutes[ref]
			mcp := c.mcpRoutes[ref]
			if len(models) == 0 && len(mcp) == 0 {
				continue
			}

			// 输入已按资源和规则排序；稳定分组只把精确模型条件提到 catch-all 前。
			slices.SortStableFunc(models, func(a, b aidataplane.ModelRoute) int {
				return cmp.Compare(len(b.Headers), len(a.Headers))
			})
			if models == nil {
				models = []aidataplane.ModelRoute{}
			}

			if mcp == nil {
				mcp = []aidataplane.MCPRoute{}
			}

			group := listeners[listener.Port]
			if group == nil {
				group = &aidataplane.Listener{Port: listener.Port, Hosts: []aidataplane.Host{}}
				listeners[listener.Port] = group
			}

			group.Hosts = append(group.Hosts, aidataplane.Host{
				Name:        path.Join("gateway", key.Namespace, key.Name, "listener", listener.Name),
				Hostname:    new(listener.Hostname),
				ModelRoutes: models,
				MCPRoutes:   mcp,
			})
		}
	}

	for _, port := range slices.Sorted(maps.Keys(listeners)) {
		config.Listeners = append(config.Listeners, *listeners[port])
	}

	for _, name := range slices.Sorted(maps.Keys(c.protocolBackends)) {
		config.Backends = append(config.Backends, c.protocolBackends[name])
	}

	return config
}

// matchingListeners 使用同空间父引用；重叠的父引用只附着一次。
func (c *aiCompilation) matchingListeners(
	namespace string,
	parents []gatewayv1alpha1.ParentReference,
) []listenerRef {
	var refs []listenerRef
	for _, gateway := range c.gateways {
		if gateway.Namespace != namespace {
			continue
		}

		for _, listener := range gateway.Spec.Listeners {
			if listener.Protocol != gatewayv1alpha1.ProtocolHTTP {
				continue
			}

			for _, parent := range parents {
				if parent.Name != gateway.Name || (parent.SectionName != "" && parent.SectionName != listener.Name) {
					continue
				}

				refs = append(refs, listenerRef{gatewayKey: client.ObjectKeyFromObject(gateway), listenerName: listener.Name})
				break
			}
		}
	}

	return refs
}

// bindNetworkBackend 只登记有效使用者；缺失或正在删除的目标返回空标识。
func (c *aiCompilation) bindNetworkBackend(key client.ObjectKey) string {
	backend := c.backends[key]
	if backend == nil {
		return ""
	}

	name := path.Join(key.Namespace, key.Name)
	if _, exists := c.networkBackends[name]; exists {
		return name
	}

	c.networkBackends[name] = true

	return name
}
