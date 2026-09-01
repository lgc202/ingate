package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// IPRestrictionPolicyStore 读写 IPRestrictionPolicy 声明式资源。
type IPRestrictionPolicyStore struct {
	*resourceStore[resource.IPRestrictionPolicy, *resource.IPRestrictionPolicy, resource.IPRestrictionPolicySpec]
}

// NewIPRestrictionPolicyStore 创建 IPRestrictionPolicy Store。
func NewIPRestrictionPolicyStore(client clientset.Interface) *IPRestrictionPolicyStore {
	return &IPRestrictionPolicyStore{resourceStore: newResourceStore(
		"IP restriction policy",
		"IP restriction policies",
		func() createResourceClient[*resource.IPRestrictionPolicy] {
			return client.GatewayV1().IPRestrictionPolicies()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.IPRestrictionPolicy, string, error) {
			resources := client.GatewayV1().IPRestrictionPolicies()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.IPRestrictionPolicySpec) *resource.IPRestrictionPolicy {
			return &resource.IPRestrictionPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindIPRestrictionPolicy),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.IPRestrictionPolicy, spec resource.IPRestrictionPolicySpec) { object.Spec = spec },
	)}
}
