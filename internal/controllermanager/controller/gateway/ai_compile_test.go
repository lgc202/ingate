package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestCompileAIGolden(t *testing.T) {
	snapshot := aiResourceSnapshot()
	before := resourceFixtures{
		gateways:    *snapshot.gateways.DeepCopy(),
		httpRoutes:  *snapshot.httpRoutes.DeepCopy(),
		backends:    *snapshot.backends.DeepCopy(),
		aiBackends:  *snapshot.aiBackends.DeepCopy(),
		modelRoutes: *snapshot.modelRoutes.DeepCopy(),
		mcpRoutes:   *snapshot.mcpRoutes.DeepCopy(),
	}
	golden, err := os.ReadFile("../../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(AI golden) error = %v", err)
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, golden); err != nil {
		t.Fatalf("Compact(AI golden) error = %v", err)
	}

	data, err := encodeTestAI(snapshot)
	if err != nil {
		t.Fatalf("encodeTestAI(golden) error = %v", err)
	}

	if !bytes.Equal(data, compact.Bytes()) {
		t.Errorf("encodeTestAI(golden) = %s, want %s", data, compact.Bytes())
	}

	if diff := cmp.Diff(before, snapshot, cmp.AllowUnexported(resourceFixtures{})); diff != "" {
		t.Errorf("encodeTestAI(golden) changed snapshot (-want +got):\n%s", diff)
	}
}

func TestCompileAIModelOrderAndBindings(t *testing.T) {
	snapshot := aiResourceSnapshot()
	snapshot.gateways.Items[0].Spec.Listeners = append(snapshot.gateways.Items[0].Spec.Listeners, listener("other", 9090, ""))
	route := &snapshot.modelRoutes.Items[0]
	route.Spec.ParentRefs = []gatewayv1alpha1.ParentReference{{Name: "public"}, {Name: "public", SectionName: "api"}}
	route.Spec.Rules[0].Matches = append(route.Spec.Rules[0].Matches,
		gatewayv1alpha1.ModelRouteMatch{},
		gatewayv1alpha1.ModelRouteMatch{Headers: []gatewayv1alpha1.HTTPHeaderMatch{{Name: "X-AI-EG-Model", Value: "other"}}},
	)
	route.Spec.Rules = append(route.Spec.Rules, gatewayv1alpha1.ModelRouteRule{
		BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "model-service", ModelNameOverride: "another"}},
	})
	older := *route.DeepCopy()
	older.Name = "all"
	older.CreationTimestamp = metav1.Unix(0, 0)
	older.Spec.Rules = []gatewayv1alpha1.ModelRouteRule{{BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "model-service"}}}}
	snapshot.modelRoutes.Items = append(snapshot.modelRoutes.Items, older)

	got := compiledAIConfig(t, snapshot)

	if len(got.Listeners) != 2 || got.Listeners[0].Port != 8080 || got.Listeners[1].Port != 9090 {
		t.Fatalf("encodeTestAI(multiple listeners) listeners = %+v, want ports 8080 and 9090", got.Listeners)
	}

	wantNames := []string{
		"team-a/coding/rule/0/match/0",
		"team-a/coding/rule/0/match/2",
		"team-a/all/rule/0/match/0",
		"team-a/coding/rule/0/match/1",
		"team-a/coding/rule/1/match/0",
	}
	for _, listener := range got.Listeners {
		if len(listener.Hosts) != 1 {
			t.Fatalf("encodeTestAI(listener %d) hosts = %+v, want one host", listener.Port, listener.Hosts)
		}

		routes := listener.Hosts[0].ModelRoutes
		var names []string
		for _, route := range routes {
			names = append(names, route.Name)
		}

		if diff := cmp.Diff(wantNames, names); diff != "" {
			t.Fatalf("encodeTestAI(listener %d) model order mismatch (-want +got):\n%s", listener.Port, diff)
		}

		if *routes[0].BackendName != *routes[1].BackendName || *routes[0].BackendName != *routes[3].BackendName {
			t.Errorf("encodeTestAI(listener %d) split one rule into different backend bindings: %+v", listener.Port, routes)
		}

		if diff := cmp.Diff([]aidataplane.HeaderMatch{{Name: "x-ai-eg-model", Type: aidataplane.HeaderMatchExact, Value: "other"}}, routes[1].Headers); diff != "" {
			t.Errorf("encodeTestAI(case-insensitive header and default type) mismatch (-want +got):\n%s", diff)
		}
	}

	wantOverrides := map[string]string{
		"team-a/model-service/route/all/rule/0/ref/0":    "",
		"team-a/model-service/route/coding/rule/0/ref/0": "code-v2",
		"team-a/model-service/route/coding/rule/1/ref/0": "another",
	}
	gotOverrides := make(map[string]string)
	for _, backend := range got.Backends {
		gotOverrides[backend.Name] = *backend.ModelNameOverride
	}

	if len(got.Backends) != 3 || len(got.NetworkBackends) != 2 {
		t.Errorf("encodeTestAI(shared references) backend/network counts = %d/%d, want 3/2", len(got.Backends), len(got.NetworkBackends))
	}

	if diff := cmp.Diff(wantOverrides, gotOverrides); diff != "" {
		t.Errorf("encodeTestAI(per-rule overrides) mismatch (-want +got):\n%s", diff)
	}
}

func TestCompileAIDefaults(t *testing.T) {
	snapshot := aiResourceSnapshot()
	snapshot.aiBackends.Items[0].Spec.Schema.Prefix = ""
	snapshot.mcpRoutes.Items[0].Spec.Path = nil
	snapshot.mcpRoutes.Items[0].Spec.BackendRefs[0].Path = nil
	snapshot.modelRoutes.Items[0].Spec.Rules[0].BackendRefs[0].ModelNameOverride = ""

	got := compiledAIConfig(t, snapshot)

	if got.Backends[0].Schema.Prefix != "/v1" || *got.Backends[0].ModelNameOverride != "" {
		t.Errorf("encodeTestAI(default model binding) backend = %+v, want /v1 and empty override", got.Backends[0])
	}

	mcp := got.Listeners[0].Hosts[0].MCPRoutes[0]
	if mcp.Path != "/mcp" || mcp.BackendPath != "/mcp" {
		t.Errorf("encodeTestAI(default MCP paths) route = %+v, want /mcp paths", mcp)
	}

	snapshot.aiBackends.Items[0].Spec.Schema.Prefix = "v2/"
	snapshot.mcpRoutes.Items[0].Spec.Path = new("/entry/")
	snapshot.mcpRoutes.Items[0].Spec.BackendRefs[0].Path = new("/stream/")

	got = compiledAIConfig(t, snapshot)

	if got.Backends[0].Schema.Prefix != "v2/" || got.Listeners[0].Hosts[0].MCPRoutes[0].Path != "/entry/" || got.Listeners[0].Hosts[0].MCPRoutes[0].BackendPath != "/stream/" {
		t.Errorf("encodeTestAI(explicit paths) = %+v, want original prefixes and trailing slashes", got)
	}
}

func TestCompileAIUnavailableBackends(t *testing.T) {
	tests := []struct {
		name             string
		change           func(*resourceFixtures)
		modelUnavailable bool
		mcpUnavailable   bool
	}{
		{
			name: "missing AIBackend",
			change: func(snapshot *resourceFixtures) {
				snapshot.aiBackends.Items = nil
			},
			modelUnavailable: true,
		},
		{
			name: "deleting AIBackend",
			change: func(snapshot *resourceFixtures) {
				snapshot.aiBackends.Items[0].DeletionTimestamp = new(metav1.Unix(3, 0))
			},
			modelUnavailable: true,
		},
		{
			name: "other namespace AIBackend",
			change: func(snapshot *resourceFixtures) {
				snapshot.aiBackends.Items[0].Namespace = "team-b"
			},
			modelUnavailable: true,
		},
		{
			name: "missing network",
			change: func(snapshot *resourceFixtures) {
				snapshot.backends.Items = nil
			},
			modelUnavailable: true,
			mcpUnavailable:   true,
		},
		{
			name: "deleting model network",
			change: func(snapshot *resourceFixtures) {
				snapshot.backends.Items[0].DeletionTimestamp = new(metav1.Unix(3, 0))
			},
			modelUnavailable: true,
		},
		{
			name: "other namespace model network",
			change: func(snapshot *resourceFixtures) {
				snapshot.backends.Items[0].Namespace = "team-b"
			},
			modelUnavailable: true,
		},
		{
			name: "deleting MCP network",
			change: func(snapshot *resourceFixtures) {
				snapshot.backends.Items[1].DeletionTimestamp = new(metav1.Unix(3, 0))
			},
			mcpUnavailable: true,
		},
		{
			name: "other namespace MCP network",
			change: func(snapshot *resourceFixtures) {
				snapshot.backends.Items[1].Namespace = "team-b"
			},
			mcpUnavailable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := aiResourceSnapshot()
			want := compiledAIConfig(t, snapshot)
			tt.change(&snapshot)
			if tt.modelUnavailable {
				want.Listeners[0].Hosts[0].ModelRoutes[0].BackendName = new("")
				want.Backends = []aidataplane.Backend{}
				want.NetworkBackends = slices.DeleteFunc(want.NetworkBackends, func(backend aidataplane.NetworkBackend) bool {
					return backend.Name == "team-a/model-network"
				})
			}

			if tt.mcpUnavailable {
				want.Listeners[0].Hosts[0].MCPRoutes[0].BackendName = new("")
				want.NetworkBackends = slices.DeleteFunc(want.NetworkBackends, func(backend aidataplane.NetworkBackend) bool {
					return backend.Name == "team-a/tools-network"
				})
			}

			got := compiledAIConfig(t, snapshot)

			assertAIContent(t, want, got)
		})
	}
}

func TestCompileAIRouteRemoval(t *testing.T) {
	tests := []struct {
		name        string
		change      func(*resourceFixtures)
		removeModel bool
		removeMCP   bool
	}{
		{
			name: "empty",
			change: func(snapshot *resourceFixtures) {
				*snapshot = resourceFixtures{}
			},
			removeModel: true,
			removeMCP:   true,
		},
		{
			name: "deleting Gateway",
			change: func(snapshot *resourceFixtures) {
				snapshot.gateways.Items[0].DeletionTimestamp = new(metav1.Unix(3, 0))
			},
			removeModel: true,
			removeMCP:   true,
		},
		{
			name: "other namespace Gateway",
			change: func(snapshot *resourceFixtures) {
				snapshot.gateways.Items[0].Namespace = "team-b"
			},
			removeModel: true,
			removeMCP:   true,
		},
		{
			name: "wrong section",
			change: func(snapshot *resourceFixtures) {
				snapshot.modelRoutes.Items[0].Spec.ParentRefs[0].SectionName = "missing"
			},
			removeModel: true,
		},
		{
			name: "missing parent",
			change: func(snapshot *resourceFixtures) {
				snapshot.mcpRoutes.Items[0].Spec.ParentRefs[0].Name = "missing"
			},
			removeMCP: true,
		},
		{
			name: "deleting ModelRoute",
			change: func(snapshot *resourceFixtures) {
				snapshot.modelRoutes.Items[0].DeletionTimestamp = new(metav1.Unix(3, 0))
			},
			removeModel: true,
		},
		{
			name: "deleting MCPRoute",
			change: func(snapshot *resourceFixtures) {
				snapshot.mcpRoutes.Items[0].DeletionTimestamp = new(metav1.Unix(3, 0))
			},
			removeMCP: true,
		},
		{
			name: "Gateway conflict",
			change: func(snapshot *resourceFixtures) {
				owner := *snapshot.gateways.Items[0].DeepCopy()
				owner.Name = "earlier"
				owner.CreationTimestamp = metav1.Unix(0, 0)
				snapshot.gateways.Items = append(snapshot.gateways.Items, owner)
			},
			removeModel: true,
			removeMCP:   true,
		},
		{
			name: "deleting Gateway releases listener",
			change: func(snapshot *resourceFixtures) {
				owner := *snapshot.gateways.Items[0].DeepCopy()
				owner.Name = "earlier"
				owner.CreationTimestamp = metav1.Unix(0, 0)
				owner.DeletionTimestamp = new(metav1.Unix(3, 0))
				snapshot.gateways.Items = append(snapshot.gateways.Items, owner)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := aiResourceSnapshot()
			want := compiledAIConfig(t, snapshot)
			tt.change(&snapshot)
			if tt.removeModel {
				want.Listeners[0].Hosts[0].ModelRoutes = []aidataplane.ModelRoute{}
				want.Backends = []aidataplane.Backend{}
				want.NetworkBackends = slices.DeleteFunc(want.NetworkBackends, func(backend aidataplane.NetworkBackend) bool {
					return backend.Name == "team-a/model-network"
				})
			}

			if tt.removeMCP {
				want.Listeners[0].Hosts[0].MCPRoutes = []aidataplane.MCPRoute{}
				want.NetworkBackends = slices.DeleteFunc(want.NetworkBackends, func(backend aidataplane.NetworkBackend) bool {
					return backend.Name == "team-a/tools-network"
				})
			}

			if tt.removeModel && tt.removeMCP {
				want.Listeners = []aidataplane.Listener{}
			}

			got := compiledAIConfig(t, snapshot)

			assertAIContent(t, want, got)
		})
	}
}

func TestCompileAIUnrelatedChanges(t *testing.T) {
	snapshot := aiResourceSnapshot()
	before, err := encodeTestAI(snapshot)
	if err != nil {
		t.Fatalf("encodeTestAI(original) error = %v", err)
	}

	snapshot.gateways.Items[0].ResourceVersion = "new"
	snapshot.aiBackends.Items[0].Spec.DisplayName = "renamed"
	snapshot.modelRoutes.Items[0].Spec.DisplayName = "renamed"
	snapshot.modelRoutes.Items[0].Spec.Rules[0].Name = "renamed"
	snapshot.modelRoutes.Items[0].Status.Parents = []gatewayv1alpha1.RouteParentStatus{{ControllerName: "other"}}
	snapshot.mcpRoutes.Items[0].Spec.DisplayName = "renamed"
	idle := backendResource("http-only")
	idle.Namespace = "team-a"
	snapshot.backends.Items = append(snapshot.backends.Items, idle)
	snapshot.httpRoutes.Items = []gatewayv1alpha1.HTTPRoute{routeResource("ordinary", 1, "public", "api", "http-only", gatewayv1alpha1.PathMatchPrefix)}
	snapshot.gateways.Items[0].Spec.Listeners = append(snapshot.gateways.Items[0].Spec.Listeners, listener("http-only", 9090, ""))

	after, err := encodeTestAI(snapshot)
	if err != nil {
		t.Fatalf("encodeTestAI(unrelated changes) error = %v", err)
	}

	if !bytes.Equal(before, after) {
		t.Errorf("encodeTestAI(unrelated changes) = %s, want unchanged payload %s", after, before)
	}
}

func TestCompileAISharedDataPlaneAndDeterminism(t *testing.T) {
	snapshot := aiResourceSnapshot()
	other := aiResourceSnapshot()
	other.gateways.Items[0].Namespace = "team-b"
	other.gateways.Items[0].Spec.Listeners[0].Hostname = "b.example.com"
	other.modelRoutes.Items[0].Namespace = "team-b"
	other.aiBackends.Items[0].Namespace = "team-b"
	other.aiBackends.Items[0].Spec.Schema.Prefix = "v2/"
	other.mcpRoutes.Items[0].Namespace = "team-b"
	for i := range other.backends.Items {
		other.backends.Items[i].Namespace = "team-b"
		other.backends.Items[i].Spec.Endpoints[0].Address = "b-" + other.backends.Items[i].Spec.Endpoints[0].Address
	}

	snapshot.gateways.Items = append(snapshot.gateways.Items, other.gateways.Items...)
	snapshot.modelRoutes.Items = append(snapshot.modelRoutes.Items, other.modelRoutes.Items...)
	snapshot.aiBackends.Items = append(snapshot.aiBackends.Items, other.aiBackends.Items...)
	snapshot.mcpRoutes.Items = append(snapshot.mcpRoutes.Items, other.mcpRoutes.Items...)
	snapshot.backends.Items = append(snapshot.backends.Items, other.backends.Items...)
	snapshot.backends.Items[0].Spec.Endpoints = []gatewayv1alpha1.Endpoint{
		{Address: "z.example.com", Port: 8081, Weight: 2},
		{Address: "a.example.com", Port: 8082, Weight: 3},
		{Address: "a.example.com", Port: 8081, Weight: 1},
	}
	beforeEndpoints := slices.Clone(snapshot.backends.Items[0].Spec.Endpoints)

	data, err := encodeTestAI(snapshot)
	if err != nil {
		t.Fatalf("encodeTestAI(shared data plane) error = %v", err)
	}

	got := compiledAIConfig(t, snapshot)

	if len(got.Listeners) != 1 || len(got.Listeners[0].Hosts) != 2 || len(got.Backends) != 2 || len(got.NetworkBackends) != 4 {
		t.Fatalf("encodeTestAI(shared data plane) = %+v, want one port, two hosts, two protocol bindings and four networks", got)
	}

	for i, namespace := range []string{"team-a", "team-b"} {
		host := got.Listeners[0].Hosts[i]
		binding := got.Backends[i]
		if len(host.ModelRoutes) != 1 || len(host.MCPRoutes) != 1 {
			t.Fatalf("encodeTestAI(%s) host = %+v, want one model and one MCP route", namespace, host)
		}

		wantBinding := namespace + "/model-service/route/coding/rule/0/ref/0"
		wantHost := "gateway/" + namespace + "/public/listener/api"
		if host.Name != wantHost || *host.ModelRoutes[0].BackendName != wantBinding ||
			binding.Name != wantBinding || binding.NetworkBackendName != namespace+"/model-network" {
			t.Errorf("encodeTestAI(%s) host/binding = %+v/%+v, want same-namespace references", namespace, host, binding)
		}

		if *host.MCPRoutes[0].BackendName != namespace+"/tools-network" {
			t.Errorf("encodeTestAI(%s) MCP backend = %q, want same-namespace reference", namespace, *host.MCPRoutes[0].BackendName)
		}
	}

	if got.Backends[1].Schema.Prefix != "v2/" || got.NetworkBackends[2].Endpoints[0].Address != "b-model.internal.example" {
		t.Errorf("encodeTestAI(team-b) selected another namespace's schema or endpoints: %+v", got)
	}

	wantEndpoints := []aidataplane.Endpoint{
		{Address: "a.example.com", Port: 8081, Weight: 1},
		{Address: "a.example.com", Port: 8082, Weight: 3},
		{Address: "z.example.com", Port: 8081, Weight: 2},
	}
	if diff := cmp.Diff(wantEndpoints, got.NetworkBackends[0].Endpoints); diff != "" {
		t.Errorf("encodeTestAI(endpoint order) mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(beforeEndpoints, snapshot.backends.Items[0].Spec.Endpoints); diff != "" {
		t.Errorf("encodeTestAI() changed snapshot endpoints (-want +got):\n%s", diff)
	}

	for i := 0; i < 3; i++ {
		slices.Reverse(snapshot.gateways.Items)
		slices.Reverse(snapshot.modelRoutes.Items)
		slices.Reverse(snapshot.aiBackends.Items)
		slices.Reverse(snapshot.mcpRoutes.Items)
		slices.Reverse(snapshot.backends.Items)
		for j := range snapshot.backends.Items {
			slices.Reverse(snapshot.backends.Items[j].Spec.Endpoints)
		}

		repeated, err := encodeTestAI(snapshot)
		if err != nil {
			t.Fatalf("encodeTestAI(reordered snapshot %d) error = %v", i, err)
		}

		if !bytes.Equal(data, repeated) {
			t.Errorf("encodeTestAI(reordered snapshot %d) = %s, want %s", i, repeated, data)
		}
	}
}

func TestCompileAIUnavailableHigherPriorityRoutes(t *testing.T) {
	snapshot := aiResourceSnapshot()
	catchAll := *snapshot.modelRoutes.Items[0].DeepCopy()
	catchAll.Name = "all"
	catchAll.CreationTimestamp = metav1.Unix(0, 0)
	catchAll.Spec.Rules[0].Matches = nil
	snapshot.modelRoutes.Items[0].Spec.Rules[0].BackendRefs[0].Name = "missing"
	snapshot.modelRoutes.Items = append(snapshot.modelRoutes.Items, catchAll)
	later := *snapshot.mcpRoutes.Items[0].DeepCopy()
	later.Name = "later"
	later.CreationTimestamp = metav1.Unix(2, 0)
	snapshot.mcpRoutes.Items[0].CreationTimestamp = metav1.Unix(1, 0)
	snapshot.mcpRoutes.Items[0].Spec.BackendRefs[0].Name = "missing"
	snapshot.mcpRoutes.Items = append(snapshot.mcpRoutes.Items, later)

	got := compiledAIConfig(t, snapshot)

	models := got.Listeners[0].Hosts[0].ModelRoutes
	if len(models) != 2 || models[0].Name != "team-a/coding/rule/0/match/0" ||
		*models[0].BackendName != "" || models[1].Name != "team-a/all/rule/0/match/0" || *models[1].BackendName == "" {
		t.Errorf("encodeTestAI(unavailable exact and valid catch-all) routes = %+v, want retained empty target before valid catch-all", models)
	}

	mcp := got.Listeners[0].Hosts[0].MCPRoutes
	if len(mcp) != 2 || mcp[0].Name != "team-a/tools" || *mcp[0].BackendName != "" ||
		mcp[1].Name != "team-a/later" || *mcp[1].BackendName == "" {
		t.Errorf("encodeTestAI(unavailable first MCP route) routes = %+v, want both candidates in creation order", mcp)
	}
}

func TestCompileAIInvalidInputs(t *testing.T) {
	tests := []struct {
		name   string
		change func(*resourceFixtures)
	}{
		{
			name: "multiple model targets",
			change: func(snapshot *resourceFixtures) {
				rule := &snapshot.modelRoutes.Items[0].Spec.Rules[0]
				rule.BackendRefs = append(rule.BackendRefs, gatewayv1alpha1.AIBackendReference{Name: "other"})
			},
		},
		{
			name: "no model target",
			change: func(snapshot *resourceFixtures) {
				snapshot.modelRoutes.Items[0].Spec.Rules[0].BackendRefs = nil
			},
		},
		{
			name: "multiple MCP targets",
			change: func(snapshot *resourceFixtures) {
				route := &snapshot.mcpRoutes.Items[0]
				route.Spec.BackendRefs = append(route.Spec.BackendRefs, gatewayv1alpha1.MCPBackendReference{Name: "other"})
			},
		},
		{
			name: "no MCP target",
			change: func(snapshot *resourceFixtures) {
				snapshot.mcpRoutes.Items[0].Spec.BackendRefs = nil
			},
		},
		{
			name: "unsupported schema",
			change: func(snapshot *resourceFixtures) {
				snapshot.aiBackends.Items[0].Spec.Schema.Name = "Anthropic"
			},
		},
		{
			name: "unknown header",
			change: func(snapshot *resourceFixtures) {
				snapshot.modelRoutes.Items[0].Spec.Rules[0].Matches[0].Headers[0].Name = "other"
			},
		},
		{
			name: "unknown match type",
			change: func(snapshot *resourceFixtures) {
				snapshot.modelRoutes.Items[0].Spec.Rules[0].Matches[0].Headers[0].Type = "RegularExpression"
			},
		},
		{
			name: "empty MCP path",
			change: func(snapshot *resourceFixtures) {
				snapshot.mcpRoutes.Items[0].Spec.Path = new("")
			},
		},
		{
			name: "empty upstream path",
			change: func(snapshot *resourceFixtures) {
				snapshot.mcpRoutes.Items[0].Spec.BackendRefs[0].Path = new("")
			},
		},
		{
			name: "empty endpoints",
			change: func(snapshot *resourceFixtures) {
				snapshot.backends.Items[0].Spec.Endpoints = nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := aiResourceSnapshot()
			tt.change(&snapshot)

			data, err := encodeTestAI(snapshot)
			if err == nil || data != nil {
				t.Errorf("encodeTestAI(%s) data/error = %s/%v, want nil data and error", tt.name, data, err)
			}
		})
	}
}

func TestCompileAIPathError(t *testing.T) {
	snapshot := aiResourceSnapshot()
	snapshot.aiBackends.Items[0].Spec.Schema.Prefix = "%zz"

	_, err := encodeTestAI(snapshot)

	if _, ok := errors.AsType[*url.Error](err); !ok {
		t.Errorf("encodeTestAI(prefix %%zz) error = %v, want wrapped *url.Error", err)
	}
}

// aiResourceSnapshot 对应共享契约 golden 的领域输入，不引入独立示例业务。

func TestCompileAIMCPModelPathConflicts(t *testing.T) {
	for _, entryPath := range []string{aidataplane.ModelChatCompletionsPath, aidataplane.ModelResponsesPath} {
		t.Run(entryPath, func(t *testing.T) {
			snapshot := aiResourceSnapshot()
			snapshot.mcpRoutes.Items[0].Spec.Path = new(entryPath)

			_, err := compileAI(snapshot.compiled())

			if err == nil || !strings.Contains(err.Error(), "conflicts with model operations") {
				t.Fatalf("compileAI(overlapping entries)=%v, want diagnostic", err)
			}

			snapshot.modelRoutes.Items = nil
			config, err := compileAI(snapshot.compiled())
			if err != nil {
				t.Fatalf("compileAI(MCP-only fixed path)=%v", err)
			}

			if config.Listeners[0].Hosts[0].MCPRoutes[0].Path != entryPath {
				t.Fatal("MCP-only entry changed path")
			}
		})
	}
}

func aiResourceSnapshot() resourceFixtures {
	gw := gatewayResource("public", 1, listener("api", 8080, "gateway.example.com"))
	gw.Namespace = "team-a"
	modelNetwork := backendResource("model-network")
	modelNetwork.Namespace = "team-a"
	modelNetwork.Spec.Endpoints[0].Address = "model.internal.example"
	toolsNetwork := backendResource("tools-network")
	toolsNetwork.Namespace = "team-a"
	toolsNetwork.Spec.Endpoints[0].Address = "tools.internal.example"
	toolsNetwork.Spec.Endpoints[0].Port = 8082

	return resourceFixtures{
		gateways: gatewayv1alpha1.GatewayList{Items: []gatewayv1alpha1.Gateway{gw}},
		backends: gatewayv1alpha1.BackendList{Items: []gatewayv1alpha1.Backend{modelNetwork, toolsNetwork}},
		aiBackends: gatewayv1alpha1.AIBackendList{Items: []gatewayv1alpha1.AIBackend{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "model-service"},
			Spec: gatewayv1alpha1.AIBackendSpec{
				BackendRef: gatewayv1alpha1.BackendReference{Name: "model-network"},
				Schema:     gatewayv1alpha1.APISchema{Name: gatewayv1alpha1.APISchemaOpenAI, Prefix: "/v1"},
			},
		}}},
		modelRoutes: gatewayv1alpha1.ModelRouteList{Items: []gatewayv1alpha1.ModelRoute{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "coding", CreationTimestamp: metav1.Unix(2, 0)},
			Spec: gatewayv1alpha1.ModelRouteSpec{
				ParentRefs: []gatewayv1alpha1.ParentReference{{Name: "public", SectionName: "api"}},
				Rules: []gatewayv1alpha1.ModelRouteRule{{
					Matches: []gatewayv1alpha1.ModelRouteMatch{{
						Headers: []gatewayv1alpha1.HTTPHeaderMatch{{
							Name:  "x-ai-eg-model",
							Type:  gatewayv1alpha1.HeaderMatchExact,
							Value: "coding",
						}},
					}},
					BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "model-service", ModelNameOverride: "code-v2"}},
				}},
			},
		}}},
		mcpRoutes: gatewayv1alpha1.MCPRouteList{Items: []gatewayv1alpha1.MCPRoute{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "tools"},
			Spec: gatewayv1alpha1.MCPRouteSpec{
				ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "public", SectionName: "api"}},
				Path:        new("/mcp"),
				BackendRefs: []gatewayv1alpha1.MCPBackendReference{{Name: "tools-network", Path: new("/stream")}},
			},
		}}},
	}
}

// encodeTestAI 验证纯编译结果可以按共享契约编码，不在生产编译路径往返 JSON。
func encodeTestAI(snapshot resourceFixtures) ([]byte, error) {
	config, err := compileAI(snapshot.compiled())
	if err != nil {
		return nil, err
	}

	return aidataplane.Encode(config)
}

func compiledAIConfig(t *testing.T, snapshot resourceFixtures) aidataplane.Config {
	t.Helper()

	data, err := encodeTestAI(snapshot)
	if err != nil {
		t.Fatalf("encodeTestAI() error = %v", err)
	}

	config, err := aidataplane.Decode(data)
	if err != nil || config == nil {
		t.Fatalf("Decode(encodeTestAI()) config/error = %+v/%v, want complete config and nil error", config, err)
	}

	return *config
}

// assertAIContent 比较完整业务内容；Decode 已验证各自 revision 与内容一致。
func assertAIContent(t *testing.T, want, got aidataplane.Config) {
	t.Helper()

	want.Revision = ""
	got.Revision = ""
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encodeTestAI() content mismatch (-want +got):\n%s", diff)
	}
}
