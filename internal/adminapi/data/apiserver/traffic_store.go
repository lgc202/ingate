package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// CallerStore 读写 Caller 声明式资源。
type CallerStore struct {
	*resourceStore[resource.Caller, *resource.Caller, resource.CallerSpec]
}

// CertificateStore 读写 Certificate 声明式资源。
type CertificateStore struct {
	*resourceStore[resource.Certificate, *resource.Certificate, resource.CertificateSpec]
}

// GatewayStore 读写 Gateway 声明式资源。
type GatewayStore struct {
	*resourceStore[resource.Gateway, *resource.Gateway, resource.GatewaySpec]
}

// RouteStore 读写 Route 声明式资源。
type RouteStore struct {
	*resourceStore[resource.Route, *resource.Route, resource.RouteSpec]
}

// UpstreamStore 读写 Upstream 声明式资源。
type UpstreamStore struct {
	*resourceStore[resource.Upstream, *resource.Upstream, resource.UpstreamSpec]
}

// NewCallerStore 创建 Caller Store。
func NewCallerStore(client clientset.Interface) *CallerStore {
	return &CallerStore{resourceStore: newResourceStore(
		"caller",
		"callers",
		func() resourceClient[*resource.Caller] {
			return client.GatewayV1().Callers()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.Caller, string, error) {
			resources := client.GatewayV1().Callers()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.CallerSpec) *resource.Caller {
			return &resource.Caller{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindCaller),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.Caller, spec resource.CallerSpec) { object.Spec = spec },
	)}
}

// NewCertificateStore 创建 Certificate Store。
func NewCertificateStore(client clientset.Interface) *CertificateStore {
	return &CertificateStore{resourceStore: newResourceStore(
		"certificate",
		"certificates",
		func() resourceClient[*resource.Certificate] {
			return client.GatewayV1().Certificates()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.Certificate, string, error) {
			resources := client.GatewayV1().Certificates()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.CertificateSpec) *resource.Certificate {
			return &resource.Certificate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindCertificate),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.Certificate, spec resource.CertificateSpec) { object.Spec = spec },
	)}
}

// NewGatewayStore 创建 Gateway Store。
func NewGatewayStore(client clientset.Interface) *GatewayStore {
	return &GatewayStore{resourceStore: newResourceStore(
		"gateway",
		"gateways",
		func() resourceClient[*resource.Gateway] {
			return client.GatewayV1().Gateways()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.Gateway, string, error) {
			resources := client.GatewayV1().Gateways()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.GatewaySpec) *resource.Gateway {
			return &resource.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindGateway),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.Gateway, spec resource.GatewaySpec) { object.Spec = spec },
	)}
}

// NewRouteStore 创建 Route Store。
func NewRouteStore(client clientset.Interface) *RouteStore {
	return &RouteStore{resourceStore: newResourceStore(
		"route",
		"routes",
		func() resourceClient[*resource.Route] {
			return client.GatewayV1().Routes()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.Route, string, error) {
			resources := client.GatewayV1().Routes()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.RouteSpec) *resource.Route {
			return &resource.Route{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindRoute),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.Route, spec resource.RouteSpec) { object.Spec = spec },
	)}
}

// NewUpstreamStore 创建 Upstream Store。
func NewUpstreamStore(client clientset.Interface) *UpstreamStore {
	return &UpstreamStore{resourceStore: newResourceStore(
		"upstream",
		"upstreams",
		func() resourceClient[*resource.Upstream] {
			return client.GatewayV1().Upstreams()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.Upstream, string, error) {
			resources := client.GatewayV1().Upstreams()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.UpstreamSpec) *resource.Upstream {
			return &resource.Upstream{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindUpstream),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.Upstream, spec resource.UpstreamSpec) { object.Spec = spec },
	)}
}
