package gateway

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	toolscache "k8s.io/client-go/tools/cache"
	informertesting "k8s.io/client-go/tools/cache/testing"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type compilationTestCache struct {
	cache.Cache
	informers map[reflect.Type]cache.Informer
}

func TestCompilationSourceInitialSync(t *testing.T) {
	for _, empty := range []bool{true, false} {
		t.Run(fmt.Sprintf("empty=%t", empty), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var fixtures resourceFixtures
				if !empty {
					fixtures = aiResourceSnapshot()
				}

				s, _ := newCompilationTestSource(t, fixtures)
				startCompilationTestSource(t, s)

				config, err := compileAI(s.readResources())
				if err != nil {
					t.Fatalf("compileAI(initial collections) error = %v, want nil", err)
				}

				if empty && (len(config.Listeners) != 0 || len(config.Backends) != 0 || len(config.NetworkBackends) != 0) {
					t.Errorf("empty source configuration = %+v, want complete empty configuration", config)
				}

				if !empty && (len(config.Listeners) != 1 || len(config.Backends) != 1 || len(config.NetworkBackends) != 2) {
					t.Errorf("initial source configuration = %+v, want listener with model and MCP backends", config)
				}
			})
		})
	}
}

func TestCompilationSourceReusesUnchangedMatches(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := aiResourceSnapshot()
		http := routeResource("ordinary", 1, "public", "api", "model-network", gatewayv1alpha1.PathMatchPrefix)
		http.Namespace = "team-a"
		otherHTTP := *http.DeepCopy()
		otherHTTP.Name = "other"
		fixtures.httpRoutes.Items = []gatewayv1alpha1.HTTPRoute{http, otherHTTP}
		otherModel := *fixtures.modelRoutes.Items[0].DeepCopy()
		otherModel.Name = "other"
		fixtures.modelRoutes.Items = append(fixtures.modelRoutes.Items, otherModel)
		otherMCP := *fixtures.mcpRoutes.Items[0].DeepCopy()
		otherMCP.Name = "other"
		fixtures.mcpRoutes.Items = append(fixtures.mcpRoutes.Items, otherMCP)
		s, producers := newCompilationTestSource(t, fixtures)
		var httpCalls, modelCalls, mcpCalls atomic.Int64
		var otherHTTPCalls, otherModelCalls, otherMCPCalls atomic.Int64
		s.httpRoutes.project = func(route *gatewayv1alpha1.HTTPRoute) *httpRoute {
			if route.Name == "other" {
				otherHTTPCalls.Add(1)
			} else {
				httpCalls.Add(1)
			}

			return compileHTTPRoute(route)
		}
		s.modelRoutes.project = func(route *gatewayv1alpha1.ModelRoute) *modelRoute {
			if route.Name == "other" {
				otherModelCalls.Add(1)
			} else {
				modelCalls.Add(1)
			}

			return compileModelRoute(route)
		}
		s.mcpRoutes.project = func(route *gatewayv1alpha1.MCPRoute) *mcpRoute {
			if route.Name == "other" {
				otherMCPCalls.Add(1)
			} else {
				mcpCalls.Add(1)
			}

			return compileMCPRoute(route)
		}
		startCompilationTestSource(t, s)
		initialBinding := *s.bindings.GetKey("team-a/model-service")

		http.Spec.Rules[0].Matches[0].Path.Value = "/v2/orders"
		producers[reflect.TypeFor[*gatewayv1alpha1.HTTPRoute]()].Modify(http.DeepCopy())
		model := fixtures.modelRoutes.Items[0].DeepCopy()
		model.Spec.Rules[0].Matches[0].Headers[0].Value = "changed"
		producers[reflect.TypeFor[*gatewayv1alpha1.ModelRoute]()].Modify(model)
		mcp := fixtures.mcpRoutes.Items[0].DeepCopy()
		mcp.Spec.Path = new("/v2/tools")
		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Modify(mcp)
		synctest.Wait()

		if httpCalls.Load() != 2 || modelCalls.Load() != 2 || mcpCalls.Load() != 2 {
			t.Errorf("changed route conversions = %d/%d/%d, want 2/2/2", httpCalls.Load(), modelCalls.Load(), mcpCalls.Load())
		}

		if got := *s.httpRoutes.projected.GetKey("team-a/ordinary"); got.rules[0].matches[0].Path != "/v2/orders" {
			t.Errorf("changed HTTP match = %+v, want /v2/orders", got.rules[0].matches[0])
		}

		if got := *s.modelRoutes.projected.GetKey("team-a/coding"); got.rules[0].matches[0].Headers[0].Value != "changed" {
			t.Errorf("changed model match = %+v, want changed model value", got.rules[0].matches[0])
		}

		if got := *s.mcpRoutes.projected.GetKey("team-a/tools"); got.match.Path != "/v2/tools" {
			t.Errorf("changed MCP match = %+v, want /v2/tools", got.match)
		}

		if got := *s.bindings.GetKey("team-a/model-service"); got != initialBinding {
			t.Error("route match changes replaced protocol binding, want same cached instance")
		}

		backend := fixtures.backends.Items[0].DeepCopy()
		backend.Spec.Endpoints[0].Address = "updated.example.com"
		producers[reflect.TypeFor[*gatewayv1alpha1.Backend]()].Modify(backend)
		synctest.Wait()

		if got := *s.bindings.GetKey("team-a/model-service"); got.network == nil || got.network.endpoints[0].Address != "updated.example.com" {
			t.Errorf("changed network binding = %+v, want updated.example.com", got)
		}

		if httpCalls.Load() != 2 || modelCalls.Load() != 2 || mcpCalls.Load() != 2 {
			t.Errorf("Backend update reran match conversions = %d/%d/%d, want unchanged 2/2/2", httpCalls.Load(), modelCalls.Load(), mcpCalls.Load())
		}

		if otherHTTPCalls.Load() != 1 || otherModelCalls.Load() != 1 || otherMCPCalls.Load() != 1 {
			t.Errorf("unrelated route conversions = %d/%d/%d, want 1/1/1", otherHTTPCalls.Load(), otherModelCalls.Load(), otherMCPCalls.Load())
		}

		producers[reflect.TypeFor[*gatewayv1alpha1.HTTPRoute]()].Delete(http.DeepCopy())
		producers[reflect.TypeFor[*gatewayv1alpha1.ModelRoute]()].Delete(model.DeepCopy())
		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Delete(mcp.DeepCopy())
		synctest.Wait()

		if s.httpRoutes.projected.GetKey("team-a/ordinary") != nil || s.modelRoutes.projected.GetKey("team-a/coding") != nil || s.mcpRoutes.projected.GetKey("team-a/tools") != nil {
			t.Error("deleted routes remain in compiled collections")
		}

		if len(s.httpRoutes.projected.List()) != 1 || len(s.modelRoutes.projected.List()) != 1 || len(s.mcpRoutes.projected.List()) != 1 {
			t.Error("route deletion removed unrelated matches, want one remaining route of each kind")
		}
	})
}

func TestCompilationSourceRejectsPartialInitialSync(t *testing.T) {
	for _, object := range []client.Object{
		&gatewayv1alpha1.Gateway{}, &gatewayv1alpha1.HTTPRoute{}, &gatewayv1alpha1.Backend{},
		&gatewayv1alpha1.AIBackend{}, &gatewayv1alpha1.ModelRoute{}, &gatewayv1alpha1.MCPRoute{},
	} {
		t.Run(fmt.Sprintf("%T", object), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, _ := newCompilationTestSource(t, resourceFixtures{})
				resourceCache := s.gateways.cache.(*compilationTestCache)
				cause := errors.New("handler registration unavailable")
				kind := reflect.TypeOf(object)
				resourceCache.informers[kind] = &registrationFailedInformer{Informer: resourceCache.informers[kind], err: cause}
				if err := s.Start(t.Context(), newResourceSourceQueue(t)); err != nil {
					t.Fatalf("Start() error = %v, want asynchronous initialization", err)
				}

				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()

				if err := s.WaitForSync(ctx); !errors.Is(err, cause) {
					t.Errorf("WaitForSync(%T failed) error = %v, want wrapped %v", object, err, cause)
				}
			})
		})
	}
}

func (c *compilationTestCache) GetInformer(
	_ context.Context,
	object client.Object,
	_ ...cache.InformerGetOption,
) (cache.Informer, error) {
	informer, found := c.informers[reflect.TypeOf(object)]
	if !found {
		return nil, fmt.Errorf("test informer for %T is not registered", object)
	}

	return informer, nil
}

func (c *compilationTestCache) WaitForCacheSync(ctx context.Context) bool {
	var synced []toolscache.InformerSynced
	for _, informer := range c.informers {
		synced = append(synced, informer.HasSynced)
	}

	return toolscache.WaitForCacheSync(ctx.Done(), synced...)
}

func newCompilationTestSource(
	t *testing.T,
	fixtures resourceFixtures,
) (*compilationSource, map[reflect.Type]*informertesting.FakeControllerSource) {
	t.Helper()
	resourceCache := &compilationTestCache{informers: make(map[reflect.Type]cache.Informer)}
	producers := make(map[reflect.Type]*informertesting.FakeControllerSource)
	for _, entry := range []struct {
		object client.Object
		list   client.ObjectList
	}{
		{object: &gatewayv1alpha1.Gateway{}, list: &fixtures.gateways},
		{object: &gatewayv1alpha1.HTTPRoute{}, list: &fixtures.httpRoutes},
		{object: &gatewayv1alpha1.Backend{}, list: &fixtures.backends},
		{object: &gatewayv1alpha1.AIBackend{}, list: &fixtures.aiBackends},
		{object: &gatewayv1alpha1.ModelRoute{}, list: &fixtures.modelRoutes},
		{object: &gatewayv1alpha1.MCPRoute{}, list: &fixtures.mcpRoutes},
	} {
		producer := informertesting.NewFakeControllerSource()
		t.Cleanup(producer.Shutdown)
		objects, err := meta.ExtractList(entry.list)
		if err != nil {
			t.Fatalf("ExtractList(%T) error = %v, want nil", entry.list, err)
		}

		for _, object := range objects {
			producer.Add(object.DeepCopyObject())
		}

		informer := toolscache.NewSharedIndexInformer(producer, entry.object, 0, nil)
		kind := reflect.TypeOf(entry.object)
		resourceCache.informers[kind] = informer
		producers[kind] = producer
		runResourceInformer(t, informer)
	}

	return newCompilationSource(resourceCache), producers
}

func startCompilationTestSource(t *testing.T, s *compilationSource) {
	t.Helper()
	if err := s.Start(t.Context(), newResourceSourceQueue(t)); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := s.WaitForSync(ctx); err != nil {
		t.Fatalf("WaitForSync() error = %v, want nil", err)
	}
}
