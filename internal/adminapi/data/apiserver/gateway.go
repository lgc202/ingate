package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// GatewayStore 读写 Gateway 声明式资源。
type GatewayStore struct {
	*resourceStore[resource.Gateway, *resource.Gateway, resource.GatewaySpec]
}

// NewGatewayStore 创建 Gateway Store。
func NewGatewayStore(client clientset.Interface) *GatewayStore {
	return &GatewayStore{resourceStore: newResourceStore(
		"gateway",
		"gateways",
		func() createResourceClient[*resource.Gateway] {
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
