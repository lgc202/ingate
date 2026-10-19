package gateway

import (
	"context"
	"testing"
	"time"

	"istio.io/istio/pkg/kube/krt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// resourceFixtures 保留 API 对象，编译测试显式转换对象结果和引用绑定。
type resourceFixtures struct {
	gateways    gatewayv1alpha1.GatewayList
	httpRoutes  gatewayv1alpha1.HTTPRouteList
	backends    gatewayv1alpha1.BackendList
	aiBackends  gatewayv1alpha1.AIBackendList
	modelRoutes gatewayv1alpha1.ModelRouteList
	mcpRoutes   gatewayv1alpha1.MCPRouteList
}

func (f resourceFixtures) compiled() resources {
	compiled := resources{
		gateways:    f.gateways,
		httpRoutes:  compileTestObjects(f.httpRoutes.Items, compileHTTPRoute),
		backends:    compileTestBackends(f.backends.Items),
		modelRoutes: compileTestObjects(f.modelRoutes.Items, compileModelRoute),
		mcpRoutes:   compileTestObjects(f.mcpRoutes.Items, compileMCPRoute),
	}
	for _, backend := range compileTestObjects(f.aiBackends.Items, compileModelBackend) {
		binding := &modelBackendBinding{backend: backend}
		for _, network := range compiled.backends {
			if backend.networkKey == network.key && !backend.deleting && !network.deleting {
				binding.network = network
			}
		}

		compiled.aiBackends = append(compiled.aiBackends, binding)
	}

	return compiled
}

// setTestResources 在 Reconcile 的只读集合边界注入完整结果，不启动 Informer。
// 对象转换、动态依赖和实际 Source 生命周期由独立测试覆盖。
func setTestResources(t *testing.T, reconciler *Reconciler, fixtures resourceFixtures) {
	t.Helper()
	compiled := fixtures.compiled()
	gateways := compileTestObjects(fixtures.gateways.Items, projectGateway)
	protocols := compileTestObjects(fixtures.aiBackends.Items, compileModelBackend)
	stop := krt.WithStop(t.Context().Done())
	reconciler.sources = &compilationSource{
		gateways: &resourceSource[*gatewayv1alpha1.Gateway, gatewayv1alpha1.Gateway]{
			projected: krt.NewStaticCollection(nil, gateways, stop),
		},
		httpRoutes: &resourceSource[*gatewayv1alpha1.HTTPRoute, httpRoute]{
			projected: krt.NewStaticCollection(nil, compiled.httpRoutes, stop),
		},
		backends: &resourceSource[*gatewayv1alpha1.Backend, networkBackend]{
			projected: krt.NewStaticCollection(nil, compiled.backends, stop),
		},
		aiBackends: &resourceSource[*gatewayv1alpha1.AIBackend, modelBackend]{
			projected: krt.NewStaticCollection(nil, protocols, stop),
		},
		modelRoutes: &resourceSource[*gatewayv1alpha1.ModelRoute, modelRoute]{
			projected: krt.NewStaticCollection(nil, compiled.modelRoutes, stop),
		},
		mcpRoutes: &resourceSource[*gatewayv1alpha1.MCPRoute, mcpRoute]{
			projected: krt.NewStaticCollection(nil, compiled.mcpRoutes, stop),
		},
		bindings: krt.NewStaticCollection(nil, compiled.aiBackends, stop),
	}
	reconciler.sources.registerEntryGraph(t.Context(), newResourceSourceQueue(t))
	if err := reconciler.sources.waitForEntries(t.Context()); err != nil {
		t.Fatalf("waitForEntries() error = %v, want nil", err)
	}

	if err := wait.PollUntilContextTimeout(t.Context(), time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
		return reconciler.sources.readPublication().err == nil, nil
	}); err != nil {
		t.Fatalf("test entry graph did not converge: %v", err)
	}
}

func compileTestObjects[T, R any](objects []T, compile func(*T) *R) []*R {
	var compiled []*R
	for i := range objects {
		compiled = append(compiled, compile(&objects[i]))
	}

	return compiled
}

func compileTestBackends(backends []gatewayv1alpha1.Backend) []*networkBackend {
	var compiled []*networkBackend
	for i := range backends {
		compiled = append(compiled, compileNetworkBackend(&backends[i]))
	}

	return compiled
}

func backendResource(name string) gatewayv1alpha1.Backend {
	return gatewayv1alpha1.Backend{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1alpha1.BackendSpec{Endpoints: []gatewayv1alpha1.Endpoint{
			{Address: "127.0.0.1", Port: 8081, Weight: 1},
		}},
	}
}
