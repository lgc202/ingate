package apiserver

import (
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// CallerStore 读写 Caller 声明式资源。
type CallerStore = resourceStore[resource.Caller, *resource.Caller, *resource.CallerList, resource.CallerSpec]

// CertificateStore 读写 Certificate 声明式资源。
type CertificateStore = resourceStore[resource.Certificate, *resource.Certificate, *resource.CertificateList, resource.CertificateSpec]

// GatewayStore 读写 Gateway 声明式资源。
type GatewayStore = resourceStore[resource.Gateway, *resource.Gateway, *resource.GatewayList, resource.GatewaySpec]

// RouteStore 读写 Route 声明式资源。
type RouteStore = resourceStore[resource.Route, *resource.Route, *resource.RouteList, resource.RouteSpec]

// UpstreamStore 读写 Upstream 声明式资源。
type UpstreamStore = resourceStore[resource.Upstream, *resource.Upstream, *resource.UpstreamList, resource.UpstreamSpec]

// NewCallerStore 创建 Caller Store。
func NewCallerStore(client clientset.Interface) *CallerStore {
	return &CallerStore{
		singularName: "caller",
		pluralName:   "callers",
		client:       client.GatewayV1().Callers(),
		unpackList: func(list *resource.CallerList) ([]resource.Caller, string) {
			return list.Items, list.Continue
		},
		newObject: func(resourceID string, spec resource.CallerSpec) *resource.Caller {
			return newResource(resourceID, resource.KindCaller, &resource.Caller{Spec: spec})
		},
		setSpec: func(object *resource.Caller, spec resource.CallerSpec) { object.Spec = spec },
	}
}

// NewCertificateStore 创建 Certificate Store。
func NewCertificateStore(client clientset.Interface) *CertificateStore {
	return &CertificateStore{
		singularName: "certificate",
		pluralName:   "certificates",
		client:       client.GatewayV1().Certificates(),
		unpackList: func(list *resource.CertificateList) ([]resource.Certificate, string) {
			return list.Items, list.Continue
		},
		newObject: func(resourceID string, spec resource.CertificateSpec) *resource.Certificate {
			return newResource(resourceID, resource.KindCertificate, &resource.Certificate{Spec: spec})
		},
		setSpec: func(object *resource.Certificate, spec resource.CertificateSpec) { object.Spec = spec },
	}
}

// NewGatewayStore 创建 Gateway Store。
func NewGatewayStore(client clientset.Interface) *GatewayStore {
	return &GatewayStore{
		singularName: "gateway",
		pluralName:   "gateways",
		client:       client.GatewayV1().Gateways(),
		unpackList: func(list *resource.GatewayList) ([]resource.Gateway, string) {
			return list.Items, list.Continue
		},
		newObject: func(resourceID string, spec resource.GatewaySpec) *resource.Gateway {
			return newResource(resourceID, resource.KindGateway, &resource.Gateway{Spec: spec})
		},
		setSpec: func(object *resource.Gateway, spec resource.GatewaySpec) { object.Spec = spec },
	}
}

// NewRouteStore 创建 Route Store。
func NewRouteStore(client clientset.Interface) *RouteStore {
	return &RouteStore{
		singularName: "route",
		pluralName:   "routes",
		client:       client.GatewayV1().Routes(),
		unpackList: func(list *resource.RouteList) ([]resource.Route, string) {
			return list.Items, list.Continue
		},
		newObject: func(resourceID string, spec resource.RouteSpec) *resource.Route {
			return newResource(resourceID, resource.KindRoute, &resource.Route{Spec: spec})
		},
		setSpec: func(object *resource.Route, spec resource.RouteSpec) { object.Spec = spec },
	}
}

// NewUpstreamStore 创建 Upstream Store。
func NewUpstreamStore(client clientset.Interface) *UpstreamStore {
	return &UpstreamStore{
		singularName: "upstream",
		pluralName:   "upstreams",
		client:       client.GatewayV1().Upstreams(),
		unpackList: func(list *resource.UpstreamList) ([]resource.Upstream, string) {
			return list.Items, list.Continue
		},
		newObject: func(resourceID string, spec resource.UpstreamSpec) *resource.Upstream {
			return newResource(resourceID, resource.KindUpstream, &resource.Upstream{Spec: spec})
		},
		setSpec: func(object *resource.Upstream, spec resource.UpstreamSpec) { object.Spec = spec },
	}
}
