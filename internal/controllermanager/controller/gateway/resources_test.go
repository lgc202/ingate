package gateway

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// resourceFixtures 保留 API 对象，便于场景构造；编译入口显式转换 Backend 网络结果。
type resourceFixtures struct {
	gateways    gatewayv1alpha1.GatewayList
	httpRoutes  gatewayv1alpha1.HTTPRouteList
	backends    gatewayv1alpha1.BackendList
	aiBackends  gatewayv1alpha1.AIBackendList
	modelRoutes gatewayv1alpha1.ModelRouteList
	mcpRoutes   gatewayv1alpha1.MCPRouteList
}

func TestReadResources(t *testing.T) {
	gateway := gatewayResource("edge", 1, listener("http", 8080, ""))
	route := routeResource("route", 2, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	backend := backendResource("backend")
	apiClient := newFakeClient(t, &gateway, &route, &backend)
	r := newTestReconciler(t, apiClient, nil, backend)

	got, err := r.readResources(t.Context())
	if err != nil {
		t.Fatalf("readResources() error = %v, want nil", err)
	}

	if diff := cmp.Diff([]gatewayv1alpha1.Gateway{gateway}, got.gateways.Items); diff != "" {
		t.Errorf("readResources() Gateways mismatch (-want +got):\n%s", diff)
	}

	if len(got.httpRoutes.Items) != 0 {
		t.Errorf("readResources() HTTPRoutes = %+v, want no protocol-specific input", got.httpRoutes.Items)
	}

	if diff := cmp.Diff(compileTestBackends([]gatewayv1alpha1.Backend{backend}), got.backends, cmp.AllowUnexported(networkBackend{})); diff != "" {
		t.Errorf("readResources() Backends mismatch (-want +got):\n%s", diff)
	}
}

func (f resourceFixtures) compiled() resources {
	return resources{
		gateways:    f.gateways,
		httpRoutes:  f.httpRoutes,
		backends:    compileTestBackends(f.backends.Items),
		aiBackends:  f.aiBackends,
		modelRoutes: f.modelRoutes,
		mcpRoutes:   f.mcpRoutes,
	}
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
