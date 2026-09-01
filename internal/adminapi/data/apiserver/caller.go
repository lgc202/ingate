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

// NewCallerStore 创建 Caller Store。
func NewCallerStore(client clientset.Interface) *CallerStore {
	return &CallerStore{resourceStore: newResourceStore(
		"caller",
		"callers",
		func() createResourceClient[*resource.Caller] {
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
