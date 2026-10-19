package gateway

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"testing/synctest"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestAIResourcesPreserveCompiledTargetsAndOrigin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		s, _ := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, s)
		synctest.Wait()
		p := s.readPublication()
		config, err := p.aiConfig()
		if err != nil {
			t.Fatalf("aiConfig() error = %v, want nil", err)
		}

		networks := make(map[string]aidataplane.NetworkBackend)
		for _, network := range config.NetworkBackends {
			networks[network.Name] = network
		}

		backend := config.Backends[0]
		modelTarget, err := aidataplane.ModelTargetName(backend, networks[backend.NetworkBackendName])
		if err != nil {
			t.Fatalf("ModelTargetName(%q) error = %v, want nil", backend.Name, err)
		}

		mcpTarget, err := aidataplane.MCPTargetName(networks["team-a/tools-network"])
		if err != nil {
			t.Fatalf("MCPTargetName(tools-network) error = %v, want nil", err)
		}

		refs := []*aidataplanev1.EntryReference{{Port: 8080, HostName: "gateway/team-a/public/listener/api"}}
		want := map[string]*aidataplanev1.Resource{
			"entry/8080/gateway/team-a/public/listener/api": {
				Name: "entry/8080/gateway/team-a/public/listener/api",
				Payload: &aidataplanev1.Resource_Entry{Entry: &aidataplanev1.Entry{
					Port: 8080, HostName: refs[0].HostName, Hostname: "gateway.example.com",
				}},
			},
			"model/team-a/coding/rule/0": {
				Name: "model/team-a/coding/rule/0",
				Payload: &aidataplanev1.Resource_ModelRule{ModelRule: &aidataplanev1.ModelRule{
					EntryRefs: refs,
					Origin: &aidataplanev1.Origin{
						Namespace:    "team-a",
						Name:         "coding",
						CreationTime: timestamppb.New(fixtures.modelRoutes.Items[0].CreationTimestamp.Time),
					},
					Matches: []*aidataplanev1.ModelMatch{{Condition: &aidataplanev1.ModelMatch_Exact{Exact: "coding"}}},
					Outcome: &aidataplanev1.ModelRule_Target{Target: &aidataplanev1.ModelTarget{
						BindingName: "team-a/model-service/route/coding/rule/0/ref/0",
						Schema: &aidataplanev1.APISchema{
							Name:   aidataplanev1.APISchemaName_API_SCHEMA_NAME_OPENAI,
							Prefix: "/v1",
						},
						ModelNameOverride:  "code-v2",
						NetworkBackendName: "team-a/model-network",
						TargetName:         modelTarget,
					}},
				}},
			},
			"mcp/team-a/tools": {
				Name: "mcp/team-a/tools",
				Payload: &aidataplanev1.Resource_McpRoute{McpRoute: &aidataplanev1.MCPRoute{
					EntryRefs: refs,
					Origin: &aidataplanev1.Origin{
						Namespace:    "team-a",
						Name:         "tools",
						CreationTime: timestamppb.New(fixtures.mcpRoutes.Items[0].CreationTimestamp.Time),
					},
					Path: "/mcp",
					Outcome: &aidataplanev1.MCPRoute_Target{Target: &aidataplanev1.MCPTarget{
						NetworkBackendName: "team-a/tools-network", BackendPath: "/stream", TargetName: mcpTarget,
					}},
				}},
			},
		}

		got := compiledAIResources(t, p)
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("aiResources() mismatch (-want +got):\n%s", diff)
		}

		// ops 只有普通 HTTPRoute，不得向 AI Proxy 下发这个入口。
		if _, ok := got["entry/9090/gateway/team-a/public/listener/ops"]; ok {
			t.Error("aiResources() included HTTP-only entry")
		}
	})
}

func TestAIResourcesMergeEntryReferencesAndPreserveMatchOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		model := &fixtures.modelRoutes.Items[0]
		model.Spec.ParentRefs[0].SectionName = ""
		model.Spec.Rules[0].Matches = append([]gatewayv1alpha1.ModelRouteMatch{{}}, model.Spec.Rules[0].Matches...)
		model.Spec.Rules = append(model.Spec.Rules, gatewayv1alpha1.ModelRouteRule{BackendRefs: model.Spec.Rules[0].BackendRefs})
		fixtures.mcpRoutes.Items[0].Spec.ParentRefs[0].SectionName = ""
		s, _ := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, s)
		synctest.Wait()
		p := s.readPublication()

		got := compiledAIResources(t, p)
		if len(got) != 5 {
			t.Fatalf("aiResources(two entries) count = %d, want 5", len(got))
		}

		wantRefs := []*aidataplanev1.EntryReference{
			{Port: 8080, HostName: "gateway/team-a/public/listener/api"},
			{Port: 9090, HostName: "gateway/team-a/public/listener/ops"},
		}
		for _, name := range []string{"model/team-a/coding/rule/0", "model/team-a/coding/rule/1"} {
			if diff := cmp.Diff(wantRefs, got[name].GetModelRule().EntryRefs, protocmp.Transform()); diff != "" {
				t.Errorf("aiResources()[%q] entry refs mismatch (-want +got):\n%s", name, diff)
			}
		}

		if diff := cmp.Diff(wantRefs, got["mcp/team-a/tools"].GetMcpRoute().EntryRefs, protocmp.Transform()); diff != "" {
			t.Errorf("aiResources()[tools] entry refs mismatch (-want +got):\n%s", diff)
		}

		wantMatches := []*aidataplanev1.ModelMatch{
			{Condition: &aidataplanev1.ModelMatch_All{All: &emptypb.Empty{}}},
			{Condition: &aidataplanev1.ModelMatch_Exact{Exact: "coding"}},
		}
		if diff := cmp.Diff(wantMatches, got["model/team-a/coding/rule/0"].GetModelRule().Matches, protocmp.Transform()); diff != "" {
			t.Errorf("aiResources()[rule/0] matches mismatch (-want +got):\n%s", diff)
		}

		second := got["model/team-a/coding/rule/1"].GetModelRule()
		if second.RuleIndex != 1 || len(second.Matches) != 1 || second.Matches[0].GetAll() == nil ||
			second.GetTarget().ModelNameOverride != "code-v2" {
			t.Errorf("aiResources()[rule/1] = %v, want index 1, default catch-all and preserved target", second)
		}

		// 表的遍历顺序不能成为资源或引用顺序；结果也不能持有可变图数据。
		p.tables = slices.Clone(p.tables)
		slices.Reverse(p.tables)
		reversed := compiledAIResources(t, p)
		if diff := cmp.Diff(got, reversed, protocmp.Transform()); diff != "" {
			t.Errorf("aiResources(reversed tables) mismatch (-want +got):\n%s", diff)
		}

		got["model/team-a/coding/rule/0"].GetModelRule().GetTarget().Schema.Prefix = "/mutated"
		got["mcp/team-a/tools"].GetMcpRoute().EntryRefs[0].HostName = "mutated"
		if diff := cmp.Diff(reversed, compiledAIResources(t, p), protocmp.Transform()); diff != "" {
			t.Errorf("aiResources(after caller mutation) mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestAIResourcesChangesRemainScoped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		s, producers := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, s)
		synctest.Wait()
		before := compiledAIResources(t, s.readPublication())

		http := &fixtures.httpRoutes.Items[0]
		http.Spec.Rules[0].Matches[0].Path.Value = "/changed"
		producers[reflect.TypeFor[*gatewayv1alpha1.HTTPRoute]()].Modify(http.DeepCopy())
		synctest.Wait()

		if diff := cmp.Diff(before, compiledAIResources(t, s.readPublication()), protocmp.Transform()); diff != "" {
			t.Errorf("aiResources(HTTP-only update) mismatch (-want +got):\n%s", diff)
		}

		model := &fixtures.modelRoutes.Items[0]
		model.Spec.Rules[0].Matches[0].Headers[0].Value = "changed-model"
		producers[reflect.TypeFor[*gatewayv1alpha1.ModelRoute]()].Modify(model.DeepCopy())
		synctest.Wait()
		after := compiledAIResources(t, s.readPublication())

		want := proto.Clone(before["model/team-a/coding/rule/0"]).(*aidataplanev1.Resource)
		want.GetModelRule().Matches[0].Condition = &aidataplanev1.ModelMatch_Exact{Exact: "changed-model"}
		before[want.Name] = want
		if diff := cmp.Diff(before, after, protocmp.Transform()); diff != "" {
			t.Errorf("aiResources(model match update) mismatch (-want +got):\n%s", diff)
		}

		network := &fixtures.backends.Items[0]
		network.Spec.Endpoints[0].Address = "changed.example.com"
		producers[reflect.TypeFor[*gatewayv1alpha1.Backend]()].Modify(network.DeepCopy())
		synctest.Wait()
		after = compiledAIResources(t, s.readPublication())

		target := after[want.Name].GetModelRule().GetTarget().TargetName
		if target == want.GetModelRule().GetTarget().TargetName {
			t.Error("aiResources(network update) retained old model target")
		}

		want.GetModelRule().GetTarget().TargetName = target
		if diff := cmp.Diff(before, after, protocmp.Transform()); diff != "" {
			t.Errorf("aiResources(network update) mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestAIResourcesKeepSameNamesInDifferentNamespacesSeparate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := aiResourceSnapshot()
		other := aiResourceSnapshot()
		other.gateways.Items[0].Namespace = "team-b"
		other.gateways.Items[0].Spec.Listeners[0].Hostname = "other.example.com"
		for i := range other.backends.Items {
			other.backends.Items[i].Namespace = "team-b"
		}

		other.aiBackends.Items[0].Namespace = "team-b"
		other.modelRoutes.Items[0].Namespace = "team-b"
		other.mcpRoutes.Items[0].Namespace = "team-b"
		fixtures.gateways.Items = append(fixtures.gateways.Items, other.gateways.Items...)
		fixtures.backends.Items = append(fixtures.backends.Items, other.backends.Items...)
		fixtures.aiBackends.Items = append(fixtures.aiBackends.Items, other.aiBackends.Items...)
		fixtures.modelRoutes.Items = append(fixtures.modelRoutes.Items, other.modelRoutes.Items...)
		fixtures.mcpRoutes.Items = append(fixtures.mcpRoutes.Items, other.mcpRoutes.Items...)
		s, _ := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, s)
		synctest.Wait()

		got := compiledAIResources(t, s.readPublication())
		if len(got) != 6 {
			t.Fatalf("aiResources(two namespaces) count = %d, want 6", len(got))
		}

		for _, namespace := range []string{"team-a", "team-b"} {
			rule := got["model/"+namespace+"/coding/rule/0"].GetModelRule()
			wantHost := "gateway/" + namespace + "/public/listener/api"
			if rule.Origin.Namespace != namespace || len(rule.EntryRefs) != 1 || rule.EntryRefs[0].HostName != wantHost ||
				rule.GetTarget().NetworkBackendName != namespace+"/model-network" {
				t.Errorf("aiResources()[%s/coding] = %v, want same-namespace origin, entry and target", namespace, rule)
			}

			mcp := got["mcp/"+namespace+"/tools"].GetMcpRoute()
			if mcp.Origin.Namespace != namespace || len(mcp.EntryRefs) != 1 || mcp.EntryRefs[0].HostName != wantHost ||
				mcp.GetTarget().NetworkBackendName != namespace+"/tools-network" {
				t.Errorf("aiResources()[%s/tools] = %v, want same-namespace origin, entry and target", namespace, mcp)
			}
		}
	})
}

func TestAIResourcesRetainUnavailableRulesAndRemoveDeletedResources(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := aiResourceSnapshot()
		fixtures.backends.Items = nil
		s, producers := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, s)
		synctest.Wait()

		got := compiledAIResources(t, s.readPublication())
		if len(got) != 3 {
			t.Fatalf("aiResources(missing backends) count = %d, want 3", len(got))
		}

		rule := got["model/team-a/coding/rule/0"].GetModelRule()
		if rule.GetUnavailable() == nil || rule.GetTarget() != nil || rule.Matches[0].GetExact() != "coding" {
			t.Errorf("aiResources(missing model backend) rule = %v, want exact unavailable rule", rule)
		}

		mcp := got["mcp/team-a/tools"].GetMcpRoute()
		if mcp.GetUnavailable() == nil || mcp.GetTarget() != nil || mcp.Path != "/mcp" {
			t.Errorf("aiResources(missing MCP backend) route = %v, want unavailable /mcp", mcp)
		}

		producers[reflect.TypeFor[*gatewayv1alpha1.ModelRoute]()].Delete(fixtures.modelRoutes.Items[0].DeepCopy())
		synctest.Wait()
		got = compiledAIResources(t, s.readPublication())

		if len(got) != 2 || got["model/team-a/coding/rule/0"] != nil || got["mcp/team-a/tools"] == nil {
			t.Errorf("aiResources(deleted model) = %v, want entry and MCP only", got)
		}

		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Delete(fixtures.mcpRoutes.Items[0].DeepCopy())
		synctest.Wait()

		if got := compiledAIResources(t, s.readPublication()); len(got) != 0 {
			t.Errorf("aiResources(deleted last route) = %v, want empty", got)
		}
	})
}

func TestAIResourcesDoNotReturnPartialCandidates(t *testing.T) {
	broken := errors.New("entry compilation failed")
	for _, tt := range []struct {
		publication publication
		wantErr     error
	}{
		{publication: publication{err: errCompilationPending}, wantErr: errCompilationPending},
		{publication: publication{tables: []*entryTable{{aiErr: broken}}}, wantErr: broken},
	} {
		got, err := tt.publication.aiResources()
		if got != nil || !errors.Is(err, tt.wantErr) {
			t.Errorf("aiResources(invalid publication) = %v, %v, want nil and %v", got, err, tt.wantErr)
		}
	}
}

func TestAIResourcesRejectInconsistentEntryResults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _ := newCompilationTestSource(t, aiResourceSnapshot())
		startCompilationTestSource(t, s)
		synctest.Wait()
		p := s.readPublication()
		// 只破坏临时候选；Entry 开始构建后，规则来源与入口内容不一致应使整批失败。
		table := *p.tables[0]
		table.ai = table.aiHost()
		table.ai.ModelRoutes = []aidataplane.ModelRoute{}
		p.tables = []*entryTable{&table}

		got, err := p.aiResources()
		if got != nil || err == nil {
			t.Errorf("aiResources(inconsistent entry) = %v, %v, want nil candidate and build error", got, err)
		}
	})
}

func compiledAIResources(t *testing.T, p publication) map[string]*aidataplanev1.Resource {
	t.Helper()
	resources, err := p.aiResources()
	if err != nil {
		t.Fatalf("aiResources() error = %v, want nil", err)
	}

	got := make(map[string]*aidataplanev1.Resource, len(resources))
	previous := ""
	for _, resource := range resources {
		if resource.Name <= previous {
			t.Fatalf("aiResources() names not unique and sorted: %q after %q", resource.Name, previous)
		}

		got[resource.Name] = resource
		previous = resource.Name
	}

	return got
}
