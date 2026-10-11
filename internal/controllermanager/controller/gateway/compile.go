package gateway

import (
	"fmt"
	"maps"
	"slices"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

type listenerRef struct {
	gatewayName  string
	listenerName string
}

// compile 只转换已接纳资源；跨资源引用和冲突已由 resolve 处理。
func compile(result resolution) dataplane.Config {
	routesByListener := make(map[listenerRef][]dataplane.Route)
	for _, binding := range result.bindings {
		ref := listenerRef{gatewayName: binding.gatewayName, listenerName: binding.listenerName}
		routesByListener[ref] = append(routesByListener[ref], dataplane.Route{
			Name:        fmt.Sprintf("%s/rule/%d/match/%d", binding.route.Name, binding.ruleIndex, binding.matchIndex),
			PathType:    dataplane.PathMatchType(binding.path.Type),
			Path:        binding.path.Value,
			BackendName: binding.backendName,
		})
	}

	listenersByPort := make(map[int32]*dataplane.Listener)
	for _, gateway := range result.gateways {
		for _, listener := range gateway.Spec.Listeners {
			group := listenersByPort[listener.Port]
			if group == nil {
				group = &dataplane.Listener{Port: listener.Port}
				listenersByPort[listener.Port] = group
			}

			ref := listenerRef{gatewayName: gateway.Name, listenerName: listener.Name}
			group.Hosts = append(group.Hosts, dataplane.Host{
				Name:     "gateway/" + gateway.Name + "/listener/" + listener.Name,
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
		endpoints := make([]dataplane.Endpoint, len(backend.Spec.Endpoints))
		for i, endpoint := range backend.Spec.Endpoints {
			endpoints[i] = dataplane.Endpoint{
				Address: endpoint.Address,
				Port:    endpoint.Port,
				Weight:  endpoint.Weight,
			}
		}
		config.Backends = append(config.Backends, dataplane.Backend{
			Name:      backend.Name,
			Endpoints: endpoints,
		})
	}

	return config
}
