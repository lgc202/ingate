package gateway

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/synctest"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestAIResourceTranslationReusesUnaffectedRules(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		model := &fixtures.modelRoutes.Items[0]
		second := model.Spec.Rules[0].DeepCopy()
		second.Matches[0].Headers[0].Value = "other-model"
		model.Spec.Rules = append(model.Spec.Rules, *second)

		source, producers := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, source)
		synctest.Wait()

		var empty aiResourceTranslation
		before := compileTestAITranslation(t, empty, source.readPublication())

		http := &fixtures.httpRoutes.Items[0]
		http.Spec.Rules[0].Matches[0].Path.Value = "/changed"
		producers[reflect.TypeFor[*gatewayv1alpha1.HTTPRoute]()].Modify(http.DeepCopy())
		synctest.Wait()

		afterHTTP := compileTestAITranslation(t, before, source.readPublication())

		for name, previous := range before.sources {
			if afterHTTP.sources[name].resource != previous.resource {
				t.Errorf("compile(HTTP update) resource %q was replaced, want reuse", name)
			}
		}

		model.Spec.Rules[0].Matches[0].Headers[0].Value = "changed-model"
		producers[reflect.TypeFor[*gatewayv1alpha1.ModelRoute]()].Modify(model.DeepCopy())
		synctest.Wait()

		afterModel := compileTestAITranslation(t, afterHTTP, source.readPublication())

		for name, previous := range afterHTTP.sources {
			changed := afterModel.sources[name].resource != previous.resource
			if want := name == "model/team-a/coding/rule/0"; changed != want {
				t.Errorf("compile(model match update) resource %q changed = %t, want %t", name, changed, want)
			}
		}

		for name, previous := range afterHTTP.modelTargets {
			if afterModel.modelTargets[name].resource != previous.resource {
				t.Errorf("compile(model match update) target %q was replaced, want reuse", name)
			}
		}
	})
}

func TestAIResourceTranslationChangesRemainScoped(t *testing.T) {
	for _, tt := range []struct {
		name     string
		change   func(*resourceFixtures) client.Object
		affected string
	}{
		{
			name: "model endpoint",
			change: func(fixtures *resourceFixtures) client.Object {
				fixtures.backends.Items[0].Spec.Endpoints[0].Address = "changed.example.com"
				return fixtures.backends.Items[0].DeepCopy()
			},
			affected: "model/team-a/coding/rule/0",
		},
		{
			name: "model override",
			change: func(fixtures *resourceFixtures) client.Object {
				fixtures.modelRoutes.Items[0].Spec.Rules[0].BackendRefs[0].ModelNameOverride = "code-v3"
				return fixtures.modelRoutes.Items[0].DeepCopy()
			},
			affected: "model/team-a/coding/rule/0",
		},
		{
			name: "protocol prefix",
			change: func(fixtures *resourceFixtures) client.Object {
				fixtures.aiBackends.Items[0].Spec.Schema.Prefix = "/provider/v1"
				return fixtures.aiBackends.Items[0].DeepCopy()
			},
			affected: "model/team-a/coding/rule/0",
		},
		{
			name: "model network reference",
			change: func(fixtures *resourceFixtures) client.Object {
				fixtures.aiBackends.Items[0].Spec.BackendRef.Name = "tools-network"
				return fixtures.aiBackends.Items[0].DeepCopy()
			},
			affected: "model/team-a/coding/rule/0",
		},
		{
			name: "MCP endpoint",
			change: func(fixtures *resourceFixtures) client.Object {
				fixtures.backends.Items[1].Spec.Endpoints[0].Address = "changed-tools.example.com"
				return fixtures.backends.Items[1].DeepCopy()
			},
			affected: "mcp/team-a/tools",
		},
		{
			name: "MCP backend path",
			change: func(fixtures *resourceFixtures) client.Object {
				fixtures.mcpRoutes.Items[0].Spec.BackendRefs[0].Path = new("/new-stream")
				return fixtures.mcpRoutes.Items[0].DeepCopy()
			},
			affected: "mcp/team-a/tools",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fixtures := twoEntryFixtures()
				source, producers := newCompilationTestSource(t, fixtures)
				startCompilationTestSource(t, source)
				synctest.Wait()

				var empty aiResourceTranslation
				before := compileTestAITranslation(t, empty, source.readPublication())

				object := tt.change(&fixtures)
				producers[reflect.TypeOf(object)].Modify(object)
				synctest.Wait()

				after := compileTestAITranslation(t, before, source.readPublication())

				for name, previous := range before.sources {
					changed := after.sources[name].resource != previous.resource
					if want := name == tt.affected; changed != want {
						t.Errorf("compile(%s) resource %q changed = %t, want %t", tt.name, name, changed, want)
					}
				}
			})
		})
	}
}

func TestAIResourceTranslationRebindsAndPrunesResources(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		source, producers := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, source)
		synctest.Wait()

		var empty aiResourceTranslation
		before := compileTestAITranslation(t, empty, source.readPublication())
		var captured []*aidataplanev1.Resource
		for _, resource := range before.resources {
			captured = append(captured, proto.CloneOf(resource))
		}

		model, mcp := &fixtures.modelRoutes.Items[0], &fixtures.mcpRoutes.Items[0]
		model.Spec.ParentRefs[0].SectionName = "ops"
		mcp.Spec.ParentRefs[0].SectionName = "ops"
		producers[reflect.TypeFor[*gatewayv1alpha1.ModelRoute]()].Modify(model.DeepCopy())
		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Modify(mcp.DeepCopy())
		synctest.Wait()

		after := compileTestAITranslation(t, before, source.readPublication())

		if after.sources["entry/8080/gateway/team-a/public/listener/api"].resource != nil ||
			after.sources["entry/9090/gateway/team-a/public/listener/ops"].resource == nil {
			t.Error("compile(rebind) did not replace the old AI entry with the new one")
		}

		wantRefs := []*aidataplanev1.EntryReference{{Port: 9090, HostName: "gateway/team-a/public/listener/ops"}}
		for name, refs := range map[string][]*aidataplanev1.EntryReference{
			"model": after.sources["model/team-a/coding/rule/0"].resource.GetModelRule().GetEntryRefs(),
			"MCP":   after.sources["mcp/team-a/tools"].resource.GetMcpRoute().GetEntryRefs(),
		} {
			if diff := cmp.Diff(wantRefs, refs, protocmp.Transform()); diff != "" {
				t.Errorf("compile(rebind) %s refs mismatch (-want +got):\n%s", name, diff)
			}
		}

		for name, previous := range before.modelTargets {
			if after.modelTargets[name].resource != previous.resource {
				t.Errorf("compile(rebind) target %q was replaced, want reuse", name)
			}
		}

		producers[reflect.TypeFor[*gatewayv1alpha1.ModelRoute]()].Delete(model.DeepCopy())
		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Delete(mcp.DeepCopy())
		synctest.Wait()

		cleared := compileTestAITranslation(t, after, source.readPublication())

		if len(cleared.resources) != 0 || len(cleared.sources) != 0 || len(cleared.modelTargets) != 0 || len(cleared.mcpTargets) != 0 {
			t.Errorf("compile(delete all) resource/source/model/MCP counts = %d/%d/%d/%d, want all zero",
				len(cleared.resources), len(cleared.sources), len(cleared.modelTargets), len(cleared.mcpTargets))
		}

		if diff := cmp.Diff(captured, before.resources, protocmp.Transform()); diff != "" {
			t.Errorf("compile(rebind and delete) mutated captured resources (-want +got):\n%s", diff)
		}
	})
}

func TestAIResourceTranslationRecoversRemovedBackend(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		source, producers := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, source)
		synctest.Wait()

		var empty aiResourceTranslation
		before := compileTestAITranslation(t, empty, source.readPublication())
		backend := &fixtures.backends.Items[0]
		producers[reflect.TypeFor[*gatewayv1alpha1.Backend]()].Delete(backend.DeepCopy())
		synctest.Wait()

		unavailable := compileTestAITranslation(t, before, source.readPublication())
		rule := unavailable.sources["model/team-a/coding/rule/0"].resource.GetModelRule()
		if rule.GetUnavailable() == nil || rule.GetTarget() != nil || len(unavailable.modelTargets) != 0 {
			t.Errorf("compile(removed model backend) rule/target count = %v/%d, want unavailable and no model target", rule, len(unavailable.modelTargets))
		}

		backend.Spec.Endpoints[0].Address = "recreated.example.com"
		producers[reflect.TypeFor[*gatewayv1alpha1.Backend]()].Add(backend.DeepCopy())
		synctest.Wait()

		recovered := compileTestAITranslation(t, unavailable, source.readPublication())
		target := recovered.sources["model/team-a/coding/rule/0"].resource.GetModelRule().GetTarget()
		oldTarget := before.sources["model/team-a/coding/rule/0"].resource.GetModelRule().GetTarget()
		if target == nil || target.TargetName == oldTarget.TargetName {
			t.Errorf("compile(recreated model backend) target = %v, want current network target instead of %q", target, oldTarget.TargetName)
		}

		if recovered.sources["mcp/team-a/tools"].resource != before.sources["mcp/team-a/tools"].resource {
			t.Error("compile(recreated model backend) replaced the unrelated MCP resource")
		}
	})
}

func TestReconcileAICacheSurvivesBuildFailureAndRecovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		source, producers := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, source)
		synctest.Wait()

		var calls int
		reconciler := New(nil, aiPublisherStub{
			publishAI: func(context.Context, aidataplane.Config) error { calls++; return nil },
		})
		if err := reconciler.reconcileAI(t.Context(), source.readPublication()); err != nil {
			t.Fatalf("reconcileAI(initial) error = %v, want nil", err)
		}

		before := reconciler.aiResources

		mcp := &fixtures.mcpRoutes.Items[0]
		mcp.Spec.Path = new(aidataplane.ModelResponsesPath)
		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Modify(mcp.DeepCopy())
		synctest.Wait()

		err := reconciler.reconcileAI(t.Context(), source.readPublication())

		if err == nil || calls != 1 {
			t.Errorf("reconcileAI(conflicting candidate) error/calls = %v/%d, want build error and one initial publish", err, calls)
		}

		for name, previous := range before.sources {
			if reconciler.aiResources.sources[name].resource != previous.resource {
				t.Errorf("reconcileAI(conflicting candidate) resource %q was replaced, want retained cache", name)
			}
		}

		mcp.Spec.Path = new("/new-tools")
		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Modify(mcp.DeepCopy())
		synctest.Wait()

		if err := reconciler.reconcileAI(t.Context(), source.readPublication()); err != nil {
			t.Fatalf("reconcileAI(recovered) error = %v, want nil", err)
		}

		path := reconciler.aiResources.sources["mcp/team-a/tools"].resource.GetMcpRoute().GetPath()
		if calls != 2 || path != "/new-tools" {
			t.Errorf("reconcileAI(recovered) calls/path = %d/%q, want 2 and /new-tools", calls, path)
		}
	})
}

func TestReconcileAICacheRetriesLatestAfterPublishFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixtures := twoEntryFixtures()
		source, producers := newCompilationTestSource(t, fixtures)
		startCompilationTestSource(t, source)
		synctest.Wait()

		cause := errors.New("AI publication failed")
		var calls int
		reconciler := New(nil, aiPublisherStub{
			publishAI: func(context.Context, aidataplane.Config) error {
				calls++
				if calls == 1 {
					return cause
				}

				return nil
			},
		})

		err := reconciler.reconcileAI(t.Context(), source.readPublication())
		if !errors.Is(err, cause) {
			t.Fatalf("reconcileAI(failed publish) error = %v, want original cause", err)
		}

		before := reconciler.aiResources

		mcp := &fixtures.mcpRoutes.Items[0]
		mcp.Spec.Path = new("/latest-tools")
		producers[reflect.TypeFor[*gatewayv1alpha1.MCPRoute]()].Modify(mcp.DeepCopy())
		synctest.Wait()

		if err := reconciler.reconcileAI(t.Context(), source.readPublication()); err != nil {
			t.Fatalf("reconcileAI(retry latest) error = %v, want nil", err)
		}

		path := reconciler.aiResources.sources["mcp/team-a/tools"].resource.GetMcpRoute().GetPath()
		if calls != 2 || path != "/latest-tools" {
			t.Errorf("reconcileAI(retry latest) calls/path = %d/%q, want 2 and /latest-tools", calls, path)
		}

		if reconciler.aiResources.sources["model/team-a/coding/rule/0"].resource != before.sources["model/team-a/coding/rule/0"].resource {
			t.Error("reconcileAI(retry latest) replaced the unchanged model resource")
		}
	})
}

// compileTestAITranslation 比较有缓存与冷启动的完整消息，再由场景断言复用边界。
func compileTestAITranslation(
	t *testing.T,
	previous aiResourceTranslation,
	snapshot publication,
) aiResourceTranslation {
	t.Helper()
	config, err := snapshot.aiConfig()
	if err != nil {
		t.Fatalf("aiConfig() error = %v, want nil", err)
	}

	next, err := previous.compile(snapshot, config)
	if err != nil {
		t.Fatalf("compile(cached AI resources) error = %v, want nil", err)
	}

	want, err := snapshot.aiResources()
	if err != nil {
		t.Fatalf("aiResources(cold compilation) error = %v, want nil", err)
	}

	if diff := cmp.Diff(want, next.resources, protocmp.Transform()); diff != "" {
		t.Fatalf("compile(cached versus cold) mismatch (-want +got):\n%s", diff)
	}

	return next
}
