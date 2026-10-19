package gateway

import (
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"istio.io/istio/pkg/kube/krt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestEntryGraphRecomputesOnlyDependentEntries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		s, producers := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, s)
		var apiCalls, opsCalls atomic.Int64
		// 使用生产节点函数和真实 Fetch 依赖计数，而非把内容相等误当成未重算。
		probe := krt.NewPointerCollection(s.entries, func(ctx krt.HandlerContext, entry *listenerEntry) *entryTable {
			if entry.listener.Name == "api" {
				apiCalls.Add(1)
			} else {
				opsCalls.Add(1)
			}

			return s.compileEntry(ctx, entry)
		}, krt.WithStop(t.Context().Done()))
		if !probe.WaitUntilSynced(t.Context().Done()) {
			t.Fatal("entry probe did not sync")
		}

		synctest.Wait()
		assertEntryConfigs(t, s, fixtures)
		assertCalls := func(api, ops int64) {
			t.Helper()
			if apiCalls.Load() != api || opsCalls.Load() != ops {
				t.Fatalf("entry transformations = %d/%d, want %d/%d", apiCalls.Load(), opsCalls.Load(), api, ops)
			}
		}
		assertCalls(1, 1)
		attachment := *s.httpAttachments.GetKey("team-a/ordinary")
		old := s.readPublication()

		route := &fixtures.httpRoutes.Items[0]
		route.Generation++
		route.Spec.Rules[0].Matches[0].Path.Value = "/changed"
		producers[reflect.TypeFor[*gatewayv1alpha1.HTTPRoute]()].Modify(route.DeepCopy())
		synctest.Wait()

		assertCalls(2, 1)
		assertEntryConfigs(t, s, fixtures)
		if *s.httpAttachments.GetKey("team-a/ordinary") != attachment {
			t.Error("match-only update replaced parent attachment")
		}

		if err := s.checkEntryVersions(old); !errors.Is(err, errCompilationPending) {
			t.Errorf("checkEntryVersions(old table) error = %v, want pending", err)
		}

		backend := &fixtures.backends.Items[0]
		backend.Spec.Endpoints[0].Address = "changed.example.com"
		producers[reflect.TypeFor[*gatewayv1alpha1.Backend]()].Modify(backend.DeepCopy())
		synctest.Wait()

		assertCalls(2, 1)
		assertEntryConfigs(t, s, fixtures)

		model := &fixtures.modelRoutes.Items[0]
		model.Spec.Rules[0].Matches[0].Headers[0].Value = "changed-model"
		producers[reflect.TypeFor[*gatewayv1alpha1.ModelRoute]()].Modify(model.DeepCopy())
		synctest.Wait()

		assertCalls(3, 1)
		assertEntryConfigs(t, s, fixtures)

		mcp := &fixtures.mcpRoutes.Items[0]
		mcp.Spec.Path = new("/changed-tools")
		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Modify(mcp.DeepCopy())
		synctest.Wait()

		assertCalls(4, 1)
		assertEntryConfigs(t, s, fixtures)

		previousOps := *s.tables.GetKey("gateway/team-a/public/listener/ops")
		route.Spec.ParentRefs[0].SectionName = "ops"
		producers[reflect.TypeFor[*gatewayv1alpha1.HTTPRoute]()].Modify(route.DeepCopy())
		synctest.Wait()

		assertEntryConfigs(t, s, fixtures)
		if apiCalls.Load() <= 4 || opsCalls.Load() <= 1 {
			t.Error("reparenting did not recompute both old and new entries")
		}

		if *s.tables.GetKey("gateway/team-a/public/listener/ops") == previousOps {
			t.Error("new parent retained its old table")
		}

		apiBefore, opsBefore := apiCalls.Load(), opsCalls.Load()
		producers[reflect.TypeFor[*gatewayv1alpha1.HTTPRoute]()].Delete(route.DeepCopy())
		fixtures.httpRoutes.Items = fixtures.httpRoutes.Items[1:]
		synctest.Wait()

		if apiCalls.Load() != apiBefore || opsCalls.Load() <= opsBefore {
			t.Error("deletion did not remain scoped to the old parent entry")
		}

		assertEntryConfigs(t, s, fixtures)
		opsBefore = opsCalls.Load()

		gateway := &fixtures.gateways.Items[0]
		gateway.Generation++
		gateway.Spec.Listeners[1].Port = 9191
		producers[reflect.TypeFor[*gatewayv1alpha1.Gateway]()].Modify(gateway.DeepCopy())
		synctest.Wait()

		assertCalls(apiBefore, opsBefore+1)
		assertEntryConfigs(t, s, fixtures)
	})
}

func TestEntryGraphRecoversMissingReferencesAndGatewayOwnership(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		missing := *fixtures.gateways.Items[0].DeepCopy()
		fixtures.gateways.Items = nil
		network := fixtures.backends.Items[0]
		fixtures.backends.Items = fixtures.backends.Items[1:]
		s, producers := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, s)
		synctest.Wait()

		assertEntryConfigs(t, s, fixtures)

		// 另一个 Namespace 的同名 Gateway 不得满足父引用。
		other := *missing.DeepCopy()
		other.Namespace = "team-b"
		fixtures.gateways.Items = append(fixtures.gateways.Items, other)
		producers[reflect.TypeFor[*gatewayv1alpha1.Gateway]()].Add(other.DeepCopy())
		synctest.Wait()

		assertEntryConfigs(t, s, fixtures)
		if got := *s.modelAttachments.GetKey("team-a/coding"); len(got.entries) != 0 {
			t.Errorf("cross-namespace attachment = %+v, want none", got.entries)
		}

		fixtures.gateways.Items = append(fixtures.gateways.Items, missing)
		producers[reflect.TypeFor[*gatewayv1alpha1.Gateway]()].Add(missing.DeepCopy())
		producers[reflect.TypeFor[*gatewayv1alpha1.Gateway]()].Delete(other.DeepCopy())
		fixtures.gateways.Items = fixtures.gateways.Items[1:]
		synctest.Wait()

		assertEntryConfigs(t, s, fixtures)
		ai, err := s.readPublication().aiConfig()
		if err != nil || len(ai.Backends) != 0 {
			t.Fatalf("missing network protocol bindings/error = %+v/%v, want empty and nil", ai.Backends, err)
		}

		fixtures.backends.Items = append(fixtures.backends.Items, network)
		producers[reflect.TypeFor[*gatewayv1alpha1.Backend]()].Add(network.DeepCopy())
		synctest.Wait()

		assertEntryConfigs(t, s, fixtures)
		ai, err = s.readPublication().aiConfig()
		if err != nil || len(ai.Backends) != 1 {
			t.Fatalf("recovered protocol bindings/error = %+v/%v, want one and nil", ai.Backends, err)
		}

		contender := *missing.DeepCopy()
		contender.Name = "later"
		contender.CreationTimestamp = metav1.NewTime(missing.CreationTimestamp.Add(time.Second))
		fixtures.gateways.Items = append(fixtures.gateways.Items, contender)
		producers[reflect.TypeFor[*gatewayv1alpha1.Gateway]()].Add(contender.DeepCopy())
		synctest.Wait()

		assertEntryConfigs(t, s, fixtures)
		if len(s.selection.Get().conflicts) != 1 {
			t.Fatal("conflicting Gateway was not rejected")
		}

		producers[reflect.TypeFor[*gatewayv1alpha1.Gateway]()].Delete(missing.DeepCopy())
		fixtures.gateways.Items = fixtures.gateways.Items[1:]
		synctest.Wait()

		assertEntryConfigs(t, s, fixtures)
		if len(s.selection.Get().conflicts) != 0 || s.entries.GetKey("gateway/team-a/later/listener/api") == nil {
			t.Error("deleting prior owner did not admit waiting Gateway")
		}
	})
}

func TestEntryPublicationDoesNotExposeCachedStorage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		s, _ := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, s)
		synctest.Wait()
		p := s.readPublication()
		http, err := p.httpConfig()
		if err != nil {
			t.Fatalf("httpConfig() error = %v", err)
		}

		ai, err := p.aiConfig()
		if err != nil {
			t.Fatalf("aiConfig() error = %v", err)
		}

		http.Listeners[0].Hosts[0].Routes[0].Path = "/mutated"
		http.Backends[0].Endpoints[0].Address = "mutated"
		*ai.Listeners[0].Hosts[0].Hostname = "mutated"
		ai.Listeners[0].Hosts[0].ModelRoutes[0].Headers[0].Value = "mutated"
		*ai.Listeners[0].Hosts[0].ModelRoutes[0].BackendName = "mutated"
		*ai.Listeners[0].Hosts[0].MCPRoutes[0].BackendName = "mutated"
		*ai.Backends[0].ModelNameOverride = "mutated"
		ai.NetworkBackends[0].Endpoints[0].Address = "mutated"

		assertEntryConfigs(t, s, fixtures)
	})
}

func TestEntryGraphChecksAICollisionsWithinEachEntry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		mcp := &fixtures.mcpRoutes.Items[0]
		mcp.Spec.Path = new("/v1/responses")
		mcp.Spec.ParentRefs[0].SectionName = "ops"
		s, producers := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, s)
		synctest.Wait()

		// 模型入口在 api，MCP 在 ops；相同路径不构成跨入口冲突。
		assertEntryConfigs(t, s, fixtures)

		mcp.Spec.ParentRefs = []gatewayv1alpha1.ParentReference{{Name: "public"}, {Name: "public", SectionName: "api"}}
		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Modify(mcp.DeepCopy())
		synctest.Wait()

		p := s.readPublication()
		if p.err != nil {
			t.Fatalf("publication error = %v, want converged tables", p.err)
		}

		if _, err := p.aiConfig(); err == nil {
			t.Error("same-entry AI collision accepted, want compilation error")
		}

		if _, err := p.httpConfig(); err != nil {
			t.Errorf("AI collision blocked HTTP configuration: %v", err)
		}

		attached := *s.mcpAttachments.GetKey("team-a/tools")
		if len(attached.entries) != 2 {
			t.Errorf("overlapping parent attachment = %+v, want exactly two entries", attached.entries)
		}

		mcp.Spec.Path = new("/mcp")
		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Modify(mcp.DeepCopy())
		synctest.Wait()

		assertEntryConfigs(t, s, fixtures)
	})
}

func twoEntryFixtures() resourceFixtures {
	fixtures := aiResourceSnapshot()
	fixtures.gateways.Items[0].Spec.Listeners = append(fixtures.gateways.Items[0].Spec.Listeners, listener("ops", 9090, "ops.example.com"))
	for i, section := range []string{"api", "api", "ops"} {
		name := []string{"ordinary", "other", "operations"}[i]
		route := routeResource(name, int64(i+3), "public", section, "model-network", gatewayv1alpha1.PathMatchPrefix)
		route.Namespace = "team-a"
		if section == "ops" {
			route.Spec.Rules[0].BackendRefs[0].Name = "tools-network"
		}

		fixtures.httpRoutes.Items = append(fixtures.httpRoutes.Items, route)
	}

	return fixtures
}

func assertEntryConfigs(t *testing.T, s *compilationSource, fixtures resourceFixtures) {
	t.Helper()
	want := fixtures.compiled()
	wantHTTP := compile(resolve(want.gateways.Items, want.httpRoutes, want.backends))
	p := s.readPublication()
	gotHTTP, err := p.httpConfig()
	if err != nil {
		t.Fatalf("httpConfig() error = %v, want nil", err)
	}

	if diff := cmp.Diff(wantHTTP, gotHTTP); diff != "" {
		t.Fatalf("entry HTTP mismatch (-want +got):\n%s", diff)
	}

	wantAI, err := compileAI(want)
	if err != nil {
		t.Fatalf("compileAI(reference) error = %v, want nil", err)
	}

	gotAI, err := p.aiConfig()
	if err != nil {
		t.Fatalf("aiConfig() error = %v, want nil", err)
	}

	if diff := cmp.Diff(wantAI, gotAI); diff != "" {
		t.Fatalf("entry AI mismatch (-want +got):\n%s", diff)
	}
}
