package index

import (
	"sync"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
)

type Index struct {
	mu    sync.RWMutex
	graph *graph
}

func New() *Index {
	return &Index{graph: newGraph()}
}

func (i *Index) UpsertGateway(obj *gatewayv1alpha1.Gateway) {
	i.withWrite(func() {
		i.graph.upsertGateway(obj)
	})
}

func (i *Index) DeleteGateway(key shared.ObjectKey) {
	i.withWrite(func() {
		i.graph.deleteGateway(key)
	})
}

func (i *Index) UpsertRoute(obj *gatewayv1alpha1.Route) {
	i.withWrite(func() {
		i.graph.upsertRoute(obj)
	})
}

func (i *Index) DeleteRoute(key shared.ObjectKey) {
	i.withWrite(func() {
		i.graph.deleteRoute(key)
	})
}

func (i *Index) UpsertBackend(obj *gatewayv1alpha1.Backend) {
	i.withWrite(func() {
		i.graph.upsertBackend(obj)
	})
}

func (i *Index) DeleteBackend(key shared.ObjectKey) {
	i.withWrite(func() {
		i.graph.deleteBackend(key)
	})
}

func (i *Index) UpsertCertificate(obj *gatewayv1alpha1.Certificate) {
	i.withWrite(func() {
		i.graph.upsertCertificate(obj)
	})
}

func (i *Index) DeleteCertificate(key shared.ObjectKey) {
	i.withWrite(func() {
		i.graph.deleteCertificate(key)
	})
}

func (i *Index) UpsertAuthPolicy(obj *policyv1alpha1.AuthPolicy) {
	i.withWrite(func() {
		i.graph.upsertAuthPolicy(obj)
	})
}

func (i *Index) DeleteAuthPolicy(key shared.ObjectKey) {
	i.withWrite(func() {
		i.graph.deleteAuthPolicy(key)
	})
}

func (i *Index) UpsertTrafficPolicy(obj *policyv1alpha1.TrafficPolicy) {
	i.withWrite(func() {
		i.graph.upsertTrafficPolicy(obj)
	})
}

func (i *Index) DeleteTrafficPolicy(key shared.ObjectKey) {
	i.withWrite(func() {
		i.graph.deleteTrafficPolicy(key)
	})
}

func (i *Index) AffectedGatewaysForGateway(key shared.ObjectKey) []shared.ObjectKey {
	return i.withRead(func() []shared.ObjectKey {
		return i.graph.affectedGatewaysForGateway(key)
	})
}

func (i *Index) AffectedGatewaysForRoute(key shared.ObjectKey) []shared.ObjectKey {
	return i.withRead(func() []shared.ObjectKey {
		return i.graph.affectedGatewaysForRoute(key)
	})
}

func (i *Index) AffectedGatewaysForBackend(key shared.ObjectKey) []shared.ObjectKey {
	return i.withRead(func() []shared.ObjectKey {
		return i.graph.affectedGatewaysForBackend(key)
	})
}

func (i *Index) AffectedGatewaysForCertificate(key shared.ObjectKey) []shared.ObjectKey {
	return i.withRead(func() []shared.ObjectKey {
		return i.graph.affectedGatewaysForCertificate(key)
	})
}

func (i *Index) AffectedGatewaysForAuthPolicy(key shared.ObjectKey) []shared.ObjectKey {
	return i.withRead(func() []shared.ObjectKey {
		return i.graph.affectedGatewaysForAuthPolicy(key)
	})
}

func (i *Index) AffectedGatewaysForTrafficPolicy(key shared.ObjectKey) []shared.ObjectKey {
	return i.withRead(func() []shared.ObjectKey {
		return i.graph.affectedGatewaysForTrafficPolicy(key)
	})
}

func (i *Index) BackendsForRoute(key shared.ObjectKey) []shared.ObjectKey {
	return i.withRead(func() []shared.ObjectKey {
		return i.graph.backendsForRoute(key)
	})
}

func (i *Index) GatewaysForBackend(key shared.ObjectKey) []shared.ObjectKey {
	return i.withRead(func() []shared.ObjectKey {
		return i.graph.gatewaysForBackend(key)
	})
}

func (i *Index) withWrite(fn func()) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.graph == nil {
		i.graph = newGraph()
	}
	fn()
}

func (i *Index) withRead(fn func() []shared.ObjectKey) []shared.ObjectKey {
	if i == nil {
		return nil
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.graph == nil {
		return nil
	}
	return fn()
}
