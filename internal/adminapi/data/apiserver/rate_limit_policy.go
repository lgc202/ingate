package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// RateLimitPolicyStore 读写 RateLimitPolicy 声明式资源。
type RateLimitPolicyStore struct {
	*resourceStore[resource.RateLimitPolicy, *resource.RateLimitPolicy, resource.RateLimitPolicySpec]
}

// NewRateLimitPolicyStore 创建 RateLimitPolicy Store。
func NewRateLimitPolicyStore(client clientset.Interface) *RateLimitPolicyStore {
	return &RateLimitPolicyStore{resourceStore: newResourceStore(
		"rate limit policy",
		"rate limit policies",
		func() createResourceClient[*resource.RateLimitPolicy] {
			return client.GatewayV1().RateLimitPolicies()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.RateLimitPolicy, string, error) {
			resources := client.GatewayV1().RateLimitPolicies()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.RateLimitPolicySpec) *resource.RateLimitPolicy {
			return &resource.RateLimitPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindRateLimitPolicy),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.RateLimitPolicy, spec resource.RateLimitPolicySpec) { object.Spec = spec },
	)}
}
