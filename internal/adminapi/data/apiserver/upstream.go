package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// UpstreamStore 读写 Upstream 声明式资源。
type UpstreamStore struct {
	*resourceStore[resource.Upstream, *resource.Upstream, resource.UpstreamSpec]
}

// NewUpstreamStore 创建 Upstream Store。
func NewUpstreamStore(client clientset.Interface) *UpstreamStore {
	return &UpstreamStore{resourceStore: newResourceStore(
		"upstream",
		"upstreams",
		func() createResourceClient[*resource.Upstream] {
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
