package gateway

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestCompileHTTPRoute(t *testing.T) {
	route := routeResource("orders", 1, "edge", "http", "orders", gatewayv1alpha1.PathMatchPrefix)
	route.Namespace = "team-a"
	route.Spec.DisplayName = "订单入口"
	route.Spec.Rules[0].Matches = []gatewayv1alpha1.HTTPRouteMatch{{}, {Path: &gatewayv1alpha1.PathMatch{Value: "/orders/"}}}
	route.Spec.Rules = append(route.Spec.Rules, gatewayv1alpha1.HTTPRouteRule{})
	before := route.DeepCopy()
	want := &httpRoute{
		ObjectMeta: projectResourceMeta(&route),
		parents:    []gatewayv1alpha1.ParentReference{{Name: "edge", SectionName: "http"}},
		rules: []httpRule{
			{
				refs: []gatewayv1alpha1.BackendReference{{Name: "orders"}},
				matches: []dataplane.Route{
					{Name: "team-a/orders/rule/0/match/0", PathType: dataplane.PathMatchPrefix, Path: "/"},
					{Name: "team-a/orders/rule/0/match/1", PathType: dataplane.PathMatchPrefix, Path: "/orders"},
				},
			},
			{matches: []dataplane.Route{{Name: "team-a/orders/rule/1/match/0", PathType: dataplane.PathMatchPrefix, Path: "/"}}},
		},
	}

	got := compileHTTPRoute(&route)

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(httpRoute{}, httpRule{})); diff != "" {
		t.Errorf("compileHTTPRoute() mismatch (-want +got):\n%s", diff)
	}

	got.parents[0].Name = "other"
	got.rules[0].refs[0].Name = "other"
	got.rules[0].matches[0].Path = "/other"
	if diff := cmp.Diff(before, &route); diff != "" {
		t.Errorf("HTTP match result shares source fields (-want +got):\n%s", diff)
	}
}

func TestCompileModelRoute(t *testing.T) {
	route := aiResourceSnapshot().modelRoutes.Items[0]
	route.Spec.Rules[0].Matches[0].Headers[0].Name = "X-AI-EG-Model"
	route.Spec.Rules[0].Matches[0].Headers[0].Type = ""
	route.Spec.Rules = append(route.Spec.Rules, gatewayv1alpha1.ModelRouteRule{})
	before := route.DeepCopy()

	want := []modelRule{
		{
			refs: []gatewayv1alpha1.AIBackendReference{{Name: "model-service", ModelNameOverride: "code-v2"}},
			matches: []aidataplane.ModelRoute{{
				Name:    "team-a/coding/rule/0/match/0",
				Headers: []aidataplane.HeaderMatch{{Name: "x-ai-eg-model", Type: aidataplane.HeaderMatchExact, Value: "coding"}},
			}},
		},
		{matches: []aidataplane.ModelRoute{{Name: "team-a/coding/rule/1/match/0", Headers: []aidataplane.HeaderMatch{}}}},
	}

	got := compileModelRoute(&route)

	if diff := cmp.Diff(want, got.rules, cmp.AllowUnexported(modelRule{})); diff != "" {
		t.Errorf("compileModelRoute() matches mismatch (-want +got):\n%s", diff)
	}

	got.parents[0].Name = "other"
	got.rules[0].refs[0].ModelNameOverride = "other"
	got.rules[0].matches[0].Headers[0].Value = "other"
	if diff := cmp.Diff(before, &route); diff != "" {
		t.Errorf("model match result shares source fields (-want +got):\n%s", diff)
	}
}

func TestCompileMCPRouteDefaults(t *testing.T) {
	tests := []struct {
		name        string
		entry       *string
		backend     *string
		wantEntry   string
		wantBackend string
	}{
		{name: "absent", wantEntry: "/mcp", wantBackend: "/mcp"},
		{name: "explicit", entry: new("/tools"), backend: new("/stream"), wantEntry: "/tools", wantBackend: "/stream"},
		{name: "empty stays invalid", entry: new(""), backend: new("")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := aiResourceSnapshot().mcpRoutes.Items[0]
			route.Spec.Path = tt.entry
			route.Spec.BackendRefs[0].Path = tt.backend
			before := route.DeepCopy()
			want := aidataplane.MCPRoute{Name: "team-a/tools", Path: tt.wantEntry, BackendPath: tt.wantBackend}

			got := compileMCPRoute(&route)

			if diff := cmp.Diff(want, got.match); diff != "" {
				t.Errorf("compileMCPRoute(%s) mismatch (-want +got):\n%s", tt.name, diff)
			}

			got.parents[0].Name = "other"
			got.refs[0].Name = "other"
			if got.refs[0].Path != nil {
				*got.refs[0].Path = "/other"
			}

			if diff := cmp.Diff(before, &route); diff != "" {
				t.Errorf("MCP match result shares source fields (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCompileModelBackend(t *testing.T) {
	backend := aiResourceSnapshot().aiBackends.Items[0]
	backend.Spec.Schema.Prefix = ""
	want := &modelBackend{
		key:        client.ObjectKey{Namespace: "team-a", Name: "model-service"},
		networkKey: client.ObjectKey{Namespace: "team-a", Name: "model-network"},
		schema:     aidataplane.APISchema{Name: aidataplane.APISchemaOpenAI, Prefix: "/v1"},
	}

	got := compileModelBackend(&backend)

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(modelBackend{})); diff != "" {
		t.Errorf("compileModelBackend(default prefix) mismatch (-want +got):\n%s", diff)
	}

	backend.Spec.Schema.Prefix = "/models"
	backend.DeletionTimestamp = new(metav1.Unix(3, 0))
	changed := compileModelBackend(&backend)
	if changed.schema.Prefix != "/models" || !changed.deleting || got.schema.Prefix != "/v1" || got.deleting {
		t.Errorf("compileModelBackend(changed) = %+v, previous = %+v, want independent explicit prefix and deletion state", changed, got)
	}
}

func TestAIObjectCompilationIgnoresDisplayChanges(t *testing.T) {
	fixtures := aiResourceSnapshot()
	model := &fixtures.modelRoutes.Items[0]
	mcp := &fixtures.mcpRoutes.Items[0]
	backend := &fixtures.aiBackends.Items[0]
	beforeModel := compileModelRoute(model)
	beforeMCP := compileMCPRoute(mcp)
	beforeBackend := compileModelBackend(backend)
	model.Generation++
	model.Spec.DisplayName = "新名称"
	mcp.Generation++
	mcp.Spec.DisplayName = "新名称"
	backend.Generation++
	backend.Spec.DisplayName = "新名称"

	if diff := cmp.Diff(beforeModel, compileModelRoute(model), cmp.AllowUnexported(modelRoute{}, modelRule{})); diff != "" {
		t.Errorf("display change affected ModelRoute matches (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(beforeMCP, compileMCPRoute(mcp), cmp.AllowUnexported(mcpRoute{})); diff != "" {
		t.Errorf("display change affected MCPRoute match (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(beforeBackend, compileModelBackend(backend), cmp.AllowUnexported(modelBackend{})); diff != "" {
		t.Errorf("display change affected AIBackend protocol (-want +got):\n%s", diff)
	}

	model.Spec.Rules[0].Matches[0].Headers[0].Value = "other"
	mcp.Spec.Path = new("/other")
	backend.Spec.Schema.Prefix = "/other"
	if cmp.Equal(beforeModel, compileModelRoute(model), cmp.AllowUnexported(modelRoute{}, modelRule{})) ||
		cmp.Equal(beforeMCP, compileMCPRoute(mcp), cmp.AllowUnexported(mcpRoute{})) ||
		cmp.Equal(beforeBackend, compileModelBackend(backend), cmp.AllowUnexported(modelBackend{})) {
		t.Error("forwarding change was suppressed, want changed match and protocol results")
	}
}

func TestPublishedConfigurationDoesNotMutateCachedMatches(t *testing.T) {
	fixtures := aiResourceSnapshot()
	route := routeResource("ordinary", 1, "public", "api", "model-network", gatewayv1alpha1.PathMatchPrefix)
	route.Namespace = "team-a"
	fixtures.httpRoutes.Items = append(fixtures.httpRoutes.Items, route)
	snapshot := fixtures.compiled()
	http := compile(resolve(snapshot.gateways.Items, snapshot.httpRoutes, snapshot.backends))
	ai, err := compileAI(snapshot)
	if err != nil {
		t.Fatalf("compileAI() error = %v, want nil", err)
	}

	expected := fixtures.compiled()
	wantHTTP := compile(resolve(expected.gateways.Items, expected.httpRoutes, expected.backends))
	wantAI, err := compileAI(expected)
	if err != nil {
		t.Fatalf("compileAI(expected) error = %v, want nil", err)
	}

	http.Listeners[0].Hosts[0].Routes[0].Path = "/changed"
	http.Backends[0].Endpoints[0].Address = "changed.example.com"
	ai.Listeners[0].Hosts[0].ModelRoutes[0].Headers[0].Value = "changed"
	*ai.Listeners[0].Hosts[0].ModelRoutes[0].BackendName = "changed"
	*ai.Listeners[0].Hosts[0].MCPRoutes[0].BackendName = "changed"
	ai.Listeners[0].Hosts[0].MCPRoutes[0].BackendPath = "/changed"
	ai.Backends[0].Schema.Prefix = "/changed"
	ai.NetworkBackends[0].Endpoints[0].Address = "changed.example.com"

	gotHTTP := compile(resolve(snapshot.gateways.Items, snapshot.httpRoutes, snapshot.backends))
	gotAI, err := compileAI(snapshot)
	if err != nil {
		t.Fatalf("compileAI(after publication mutation) error = %v, want nil", err)
	}

	if diff := cmp.Diff(wantHTTP, gotHTTP); diff != "" {
		t.Errorf("HTTP publication modified cached matches (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(wantAI, gotAI); diff != "" {
		t.Errorf("AI publication modified cached matches (-want +got):\n%s", diff)
	}
}
