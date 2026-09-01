package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// HeaderTransformationPolicyStore 读写 HeaderTransformationPolicy 声明式资源。
type HeaderTransformationPolicyStore struct {
	*resourceStore[resource.HeaderTransformationPolicy, *resource.HeaderTransformationPolicy, resource.HeaderTransformationPolicySpec]
}

// NewHeaderTransformationPolicyStore 创建 HeaderTransformationPolicy Store。
func NewHeaderTransformationPolicyStore(client clientset.Interface) *HeaderTransformationPolicyStore {
	return &HeaderTransformationPolicyStore{resourceStore: newResourceStore(
		"header transformation policy",
		"header transformation policies",
		func() createResourceClient[*resource.HeaderTransformationPolicy] {
			return client.GatewayV1().HeaderTransformationPolicies()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.HeaderTransformationPolicy, string, error) {
			resources := client.GatewayV1().HeaderTransformationPolicies()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.HeaderTransformationPolicySpec) *resource.HeaderTransformationPolicy {
			return &resource.HeaderTransformationPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindHeaderTransformationPolicy),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.HeaderTransformationPolicy, spec resource.HeaderTransformationPolicySpec) {
			object.Spec = spec
		},
	)}
}
