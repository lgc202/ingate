package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// TokenQuotaPolicyStore 读写 TokenQuotaPolicy 声明式资源。
type TokenQuotaPolicyStore struct {
	*resourceStore[resource.TokenQuotaPolicy, *resource.TokenQuotaPolicy, resource.TokenQuotaPolicySpec]
}

// NewTokenQuotaPolicyStore 创建 TokenQuotaPolicy Store。
func NewTokenQuotaPolicyStore(client clientset.Interface) *TokenQuotaPolicyStore {
	return &TokenQuotaPolicyStore{resourceStore: newResourceStore(
		"token quota policy",
		"token quota policies",
		func() createResourceClient[*resource.TokenQuotaPolicy] {
			return client.GatewayV1().TokenQuotaPolicies()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.TokenQuotaPolicy, string, error) {
			resources := client.GatewayV1().TokenQuotaPolicies()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.TokenQuotaPolicySpec) *resource.TokenQuotaPolicy {
			return &resource.TokenQuotaPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindTokenQuotaPolicy),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.TokenQuotaPolicy, spec resource.TokenQuotaPolicySpec) { object.Spec = spec },
	)}
}
