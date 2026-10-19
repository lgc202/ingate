package gateway

import (
	"cmp"
	"path"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// networkBackend 是按 Backend 独立派生的只读网络结果，不携带展示名称、代次或状态。
// 保留删除标记，让 HTTP 与 AI 继续分别执行已有的后端删除语义。
type networkBackend struct {
	key       client.ObjectKey
	endpoints []dataplane.Endpoint
	tls       *gatewayv1alpha1.BackendTLSSettings
	deleting  bool
}

func compileNetworkBackend(backend *gatewayv1alpha1.Backend) *networkBackend {
	endpoints := make([]dataplane.Endpoint, len(backend.Spec.Endpoints))
	for i, endpoint := range backend.Spec.Endpoints {
		endpoints[i] = dataplane.Endpoint{Address: endpoint.Address, Port: endpoint.Port, Weight: endpoint.Weight}
	}

	// 端点顺序不参与转发语义，规范顺序使等价重排停止在这一层。
	slices.SortFunc(endpoints, func(a, b dataplane.Endpoint) int {
		if order := cmp.Compare(a.Address, b.Address); order != 0 {
			return order
		}

		if order := cmp.Compare(a.Port, b.Port); order != 0 {
			return order
		}

		return cmp.Compare(a.Weight, b.Weight)
	})

	return &networkBackend{
		key:       client.ObjectKeyFromObject(backend),
		endpoints: endpoints,
		tls:       backend.Spec.TLS.DeepCopy(),
		deleting:  !backend.DeletionTimestamp.IsZero(),
	}
}

// ResourceName 以 Namespace 和名称标识 krt 对象，与两路输出的网络引用保持一致。
func (b *networkBackend) ResourceName() string {
	return path.Join(b.key.Namespace, b.key.Name)
}

// httpBackend 返回归本次发布所有的副本，发布器不能修改集合中的缓存结果。
func (b *networkBackend) httpBackend() dataplane.Backend {
	var tls *dataplane.BackendTLSSettings
	if b.tls != nil {
		tls = &dataplane.BackendTLSSettings{SNI: b.tls.SNI}
	}

	return dataplane.Backend{Name: b.ResourceName(), Endpoints: slices.Clone(b.endpoints), TLS: tls}
}

func (b *networkBackend) aiBackend() aidataplane.NetworkBackend {
	endpoints := make([]aidataplane.Endpoint, len(b.endpoints))
	for i, endpoint := range b.endpoints {
		endpoints[i] = aidataplane.Endpoint{Address: endpoint.Address, Port: endpoint.Port, Weight: endpoint.Weight}
	}

	var tls *aidataplane.BackendTLSSettings
	if b.tls != nil {
		tls = &aidataplane.BackendTLSSettings{
			SNI:                     b.tls.SNI,
			WellKnownCACertificates: b.tls.WellKnownCACertificates,
		}
	}

	return aidataplane.NetworkBackend{Name: b.ResourceName(), Endpoints: endpoints, TLS: tls}
}
