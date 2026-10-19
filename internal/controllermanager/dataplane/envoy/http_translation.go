package envoy

import (
	"slices"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

// httpTranslation 是最近有效的 HTTP 转换结果，发布器在持锁时读取并整体替换。
// 每次构建只保留本次声明的键，不保留删除对象或历史版本；旧结果始终只读。
// LDS 与 RDS 分开缓存，改路径不重建 Listener；CDS/EDS 随对应 Backend 一起转换，
// 保持 DNS 内嵌端点与独立 EDS 两种形态的切换一致。source 保存调用方声明的独立副本。
type httpTranslation struct {
	resources resourceSet
	listeners map[int32]*listenerv3.Listener
	routes    map[int32]httpRouteTranslation
	backends  map[string]httpBackendTranslation
}

type httpRouteTranslation struct {
	source   dataplane.Listener
	resource *routev3.RouteConfiguration
}

type httpBackendTranslation struct {
	source    dataplane.Backend
	cluster   *clusterv3.Cluster
	endpoints *endpointv3.ClusterLoadAssignment
}

// compile 只构建候选；与 AI 合并及引用校验成功后，Publisher 才保存这份结果。
func (t *httpTranslation) compile(config dataplane.Config) (httpTranslation, error) {
	next := httpTranslation{
		listeners: make(map[int32]*listenerv3.Listener, len(config.Listeners)),
		routes:    make(map[int32]httpRouteTranslation, len(config.Listeners)),
		backends:  make(map[string]httpBackendTranslation, len(config.Backends)),
	}
	for _, listener := range config.Listeners {
		route := t.routes[listener.Port]
		if route.resource == nil || !sameHTTPListener(route.source, listener) {
			resource, err := newRouteConfiguration(listener)
			if err != nil {
				return httpTranslation{}, err
			}

			route = httpRouteTranslation{source: cloneHTTPListener(listener), resource: resource}
		}

		resource := t.listeners[listener.Port]
		if resource == nil {
			var err error
			resource, err = newListener(listener.Port)
			if err != nil {
				return httpTranslation{}, err
			}
		}

		next.routes[listener.Port] = route
		next.listeners[listener.Port] = resource
		next.resources.routes = append(next.resources.routes, route.resource)
		next.resources.listeners = append(next.resources.listeners, resource)
	}

	for _, backend := range config.Backends {
		translated := t.backends[backend.Name]
		if translated.cluster == nil || !sameHTTPBackend(translated.source, backend) {
			cluster, endpoints, err := newCluster(backend)
			if err != nil {
				return httpTranslation{}, err
			}

			translated = httpBackendTranslation{source: cloneHTTPBackend(backend), cluster: cluster, endpoints: endpoints}
		}

		next.backends[backend.Name] = translated
		next.resources.clusters = append(next.resources.clusters, translated.cluster)
		if translated.cluster.GetType() == clusterv3.Cluster_EDS {
			next.resources.endpoints = append(next.resources.endpoints, translated.endpoints)
		}
	}

	return next, nil
}

func sameHTTPListener(a, b dataplane.Listener) bool {
	return a.Port == b.Port && slices.EqualFunc(a.Hosts, b.Hosts, func(a, b dataplane.Host) bool {
		return a.Name == b.Name && a.Hostname == b.Hostname && slices.Equal(a.Routes, b.Routes)
	})
}

func cloneHTTPListener(listener dataplane.Listener) dataplane.Listener {
	listener.Hosts = slices.Clone(listener.Hosts)
	for i := range listener.Hosts {
		listener.Hosts[i].Routes = slices.Clone(listener.Hosts[i].Routes)
	}

	return listener
}

func sameHTTPBackend(a, b dataplane.Backend) bool {
	if a.Name != b.Name || !slices.Equal(a.Endpoints, b.Endpoints) {
		return false
	}

	if a.TLS == nil || b.TLS == nil {
		return a.TLS == b.TLS
	}

	return *a.TLS == *b.TLS
}

func cloneHTTPBackend(backend dataplane.Backend) dataplane.Backend {
	backend.Endpoints = slices.Clone(backend.Endpoints)
	if backend.TLS != nil {
		backend.TLS = new(*backend.TLS)
	}

	return backend
}
