package gateway

import (
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
	config := dataplane.Config{Listeners: compileHTTPListeners(result)}
	for _, backend := range result.backends {
		config.Backends = append(config.Backends, backend.httpBackend())
	}

	return config
}

func compileHTTPListeners(result resolution) []dataplane.Listener {
	routesByListener := make(map[listenerRef][]dataplane.Route)
	for _, binding := range result.bindings {
		ref := listenerRef{gatewayKey: binding.gatewayKey, listenerName: binding.listenerName}
		match := binding.match
		match.BackendName = binding.backendName
		routesByListener[ref] = append(routesByListener[ref], match)
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

	var listeners []dataplane.Listener
	for _, port := range slices.Sorted(maps.Keys(listenersByPort)) {
		listeners = append(listeners, *listenersByPort[port])
	}

	return listeners
}
