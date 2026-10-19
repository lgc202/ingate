package gateway

import (
	"fmt"
	"maps"
	"path"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

type listenerRef struct {
	gatewayKey   client.ObjectKey
	listenerName string
}

// compile 只转换已接纳资源；跨资源引用和冲突已由 resolve 处理。
func compile(result resolution) dataplane.Config {
	routesByListener := make(map[listenerRef][]dataplane.Route)
	for _, binding := range result.bindings {
		ref := listenerRef{gatewayKey: binding.gatewayKey, listenerName: binding.listenerName}
		routeKey := client.ObjectKeyFromObject(binding.route)
		routeName := path.Join(routeKey.Namespace, routeKey.Name)
		routesByListener[ref] = append(routesByListener[ref], dataplane.Route{
			Name:        fmt.Sprintf("%s/rule/%d/match/%d", routeName, binding.ruleIndex, binding.matchIndex),
			PathType:    dataplane.PathMatchType(binding.path.Type),
			Path:        binding.path.Value,
			BackendName: binding.backendName,
		})
	}

	listenersByPort := make(map[int32]*dataplane.Listener)
	for _, gateway := range result.gateways {
		gatewayKey := client.ObjectKeyFromObject(gateway)
		for _, listener := range gateway.Spec.Listeners {
			group := listenersByPort[listener.Port]
			if group == nil {
				group = &dataplane.Listener{Port: listener.Port}
				listenersByPort[listener.Port] = group
			}

			ref := listenerRef{gatewayKey: gatewayKey, listenerName: listener.Name}
			group.Hosts = append(group.Hosts, dataplane.Host{
				Name:     path.Join("gateway", gatewayKey.Namespace, gatewayKey.Name, "listener", listener.Name),
				Hostname: listener.Hostname,
				Routes:   routesByListener[ref],
			})
		}
	}

	var config dataplane.Config
	for _, port := range slices.Sorted(maps.Keys(listenersByPort)) {
		config.Listeners = append(config.Listeners, *listenersByPort[port])
	}

	for _, backend := range result.backends {
		config.Backends = append(config.Backends, backend.httpBackend())
	}

	return config
}
