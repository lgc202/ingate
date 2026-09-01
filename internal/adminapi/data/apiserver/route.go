package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// RouteStore 读写 Route 声明式资源。
type RouteStore struct {
	*resourceStore[resource.Route, *resource.Route, resource.RouteSpec]
}

// NewRouteStore 创建 Route Store。
func NewRouteStore(client clientset.Interface) *RouteStore {
	return &RouteStore{resourceStore: newResourceStore(
		"route",
		"routes",
		func() createResourceClient[*resource.Route] {
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
