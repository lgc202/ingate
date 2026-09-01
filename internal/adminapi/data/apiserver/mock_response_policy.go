package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// MockResponsePolicyStore 读写 MockResponsePolicy 声明式资源。
type MockResponsePolicyStore struct {
	*resourceStore[resource.MockResponsePolicy, *resource.MockResponsePolicy, resource.MockResponsePolicySpec]
}

// NewMockResponsePolicyStore 创建 MockResponsePolicy Store。
func NewMockResponsePolicyStore(client clientset.Interface) *MockResponsePolicyStore {
	return &MockResponsePolicyStore{resourceStore: newResourceStore(
		"mock response policy",
		"mock response policies",
		func() createResourceClient[*resource.MockResponsePolicy] {
			return client.GatewayV1().MockResponsePolicies()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.MockResponsePolicy, string, error) {
			resources := client.GatewayV1().MockResponsePolicies()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.MockResponsePolicySpec) *resource.MockResponsePolicy {
			return &resource.MockResponsePolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindMockResponsePolicy),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.MockResponsePolicy, spec resource.MockResponsePolicySpec) { object.Spec = spec },
	)}
}
