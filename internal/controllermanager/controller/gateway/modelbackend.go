package gateway

import (
	"path"

	"istio.io/istio/pkg/kube/krt"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// modelBackend 是协议声明；端点由网络集合唯一拥有，不复制到每条模型匹配中。
type modelBackend struct {
	key        client.ObjectKey
	networkKey client.ObjectKey
	schema     aidataplane.APISchema
	deleting   bool
}

// modelBackendBinding 记录协议与网络结果的关联；network 为 nil 表示引用暂不可用。
// 保留不可用对象而非丢弃它，路由仍可保留匹配并拒绝请求。
type modelBackendBinding struct {
	backend *modelBackend
	network *networkBackend
}

func compileModelBackend(backend *gatewayv1alpha1.AIBackend) *modelBackend {
	prefix := backend.Spec.Schema.Prefix
	if backend.Spec.Schema.Name == gatewayv1alpha1.APISchemaOpenAI && prefix == "" {
		prefix = "/v1"
	}

	return &modelBackend{
		key:        client.ObjectKeyFromObject(backend),
		networkKey: client.ObjectKey{Namespace: backend.Namespace, Name: backend.Spec.BackendRef.Name},
		schema:     aidataplane.APISchema{Name: aidataplane.APISchemaName(backend.Spec.Schema.Name), Prefix: prefix},
		deleting:   !backend.DeletionTimestamp.IsZero(),
	}
}

// ResourceName 以 Namespace 和名称标识协议声明。
func (b *modelBackend) ResourceName() string {
	return path.Join(b.key.Namespace, b.key.Name)
}

// ResourceName 让绑定沿用协议声明的身份，网络目标变化不改变该身份。
func (b *modelBackendBinding) ResourceName() string {
	return b.backend.ResourceName()
}

func bindModelBackend(
	ctx krt.HandlerContext,
	backend *modelBackend,
	networks krt.Collection[*networkBackend],
) *modelBackendBinding {
	bound := &modelBackendBinding{backend: backend}
	if backend.deleting {
		return bound
	}

	// 即使当前没有目标也要 Fetch，krt 才能在目标创建或同名重建时恢复引用。
	network := krt.FetchOne(ctx, networks, krt.FilterKey(path.Join(backend.networkKey.Namespace, backend.networkKey.Name)))
	if network != nil && !(*network).deleting {
		bound.network = *network
	}

	return bound
}
