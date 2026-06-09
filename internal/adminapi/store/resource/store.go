package resource

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Kind 表示 admin-api 暴露的声明式资源集合
type Kind string

const (
	// KindRateLimitPolicies 表示限流策略资源
	KindRateLimitPolicies Kind = "rate-limit-policies"
	// KindAccessControlPolicies 表示访问控制策略资源
	KindAccessControlPolicies Kind = "access-control-policies"
	// KindRedisStores 表示 Redis 配置资源
	KindRedisStores Kind = "redis-stores"
	// KindPolicyBindings 表示策略绑定资源
	KindPolicyBindings Kind = "policy-bindings"
)

// Store 读取 admin-api 尚未拆出专用用例的声明式资源
type Store struct {
	client clientset.Interface
}

// New 创建声明式资源 store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询资源列表
func (s *Store) List(ctx context.Context, kind Kind) (any, error) {
	client := s.client.GatewayV1()
	switch kind {
	case KindRateLimitPolicies:
		return client.RateLimitPolicies().List(ctx, metav1.ListOptions{})
	case KindAccessControlPolicies:
		return client.AccessControlPolicies().List(ctx, metav1.ListOptions{})
	case KindRedisStores:
		return client.RedisStores().List(ctx, metav1.ListOptions{})
	case KindPolicyBindings:
		return client.PolicyBindings().List(ctx, metav1.ListOptions{})
	default:
		return nil, fmt.Errorf("unsupported admin resource kind %q", kind)
	}
}

// Get 查询单个资源
func (s *Store) Get(ctx context.Context, kind Kind, name string) (any, error) {
	client := s.client.GatewayV1()
	switch kind {
	case KindRateLimitPolicies:
		return client.RateLimitPolicies().Get(ctx, name, metav1.GetOptions{})
	case KindAccessControlPolicies:
		return client.AccessControlPolicies().Get(ctx, name, metav1.GetOptions{})
	case KindRedisStores:
		return client.RedisStores().Get(ctx, name, metav1.GetOptions{})
	case KindPolicyBindings:
		return client.PolicyBindings().Get(ctx, name, metav1.GetOptions{})
	default:
		return nil, fmt.Errorf("unsupported admin resource kind %q", kind)
	}
}
