package aiconfig

import (
	"errors"
	"io"
	"log/slog"
	"maps"
	"sync"
	"testing"
	"time"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
)

func TestResourceBatchUpdatesOnlyAffectedEntry(t *testing.T) {
	loader := NewLoader(nil, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	entry, rule := testResources()
	otherEntry, otherRule := testResources()
	otherEntry.Name, otherEntry.GetEntry().HostName = "other-entry", "other-host"
	otherRule.Name, otherRule.GetModelRule().EntryRefs[0].HostName = "other-rule", "other-host"
	applyTestBatch(t, loader, entry, rule, otherEntry, otherRule)
	previous, _ := loader.Capture()
	before, err := previous.ResolveModel(8080, "host", "coding")
	if err != nil {
		t.Fatalf("ResolveModel(initial) error = %v", err)
	}

	// 更新一个规则，不改变另一个入口的视图指针；已捕获视图继续交付旧目标。
	rule.GetModelRule().GetTarget().ModelNameOverride = "code-v3"
	applyTestBatch(t, loader, rule)
	current, _ := loader.Capture()
	after, err := current.ResolveModel(8080, "host", "coding")
	if err != nil || after.ModelNameOverride != "code-v3" {
		t.Fatalf("ResolveModel(updated) = (%+v, %v), want code-v3", after, err)
	}

	if current.entries[hostKey{port: 8080, name: "other-host"}] != previous.entries[hostKey{port: 8080, name: "other-host"}] {
		t.Error("apply(one rule) rebuilt an unrelated entry")
	}

	stillBefore, err := previous.ResolveModel(8080, "host", "coding")
	if err != nil || stillBefore != before {
		t.Errorf("ResolveModel(captured view) = (%+v, %v), want unchanged %+v", stillBefore, err, before)
	}

	// 调用方修改 protobuf 不得影响已经解码、应用的视图。
	rule.GetModelRule().GetTarget().ModelNameOverride = "caller mutation"
	got, err := current.ResolveModel(8080, "host", "coding")
	if err != nil || got != after {
		t.Errorf("ResolveModel(after source mutation) = (%+v, %v), want %+v", got, err, after)
	}

	unchanged := proto.CloneOf(rule)
	unchanged.GetModelRule().GetTarget().ModelNameOverride = "code-v3"
	applyTestBatch(t, loader, unchanged)
	if got, _ := loader.Capture(); got != current {
		t.Error("apply(identical contents) replaced the active view")
	}
}

func TestResourceBatchRejectsInvalidWithoutChangingAcceptedState(t *testing.T) {
	entry, rule := testResources()
	for _, tt := range []struct {
		name   string
		change func(*discoveryv3.DeltaDiscoveryResponse)
	}{
		{name: "wrong type", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.TypeUrl = "unknown" }},
		{name: "missing nonce", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.Nonce = "" }},
		{name: "missing version", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.Resources[0].Version = "" }},
		{name: "missing payload", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.Resources[0].Resource = nil }},
		{name: "name mismatch", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.Resources[0].Name = "other" }},
		{name: "duplicate update", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.Resources = append(r.Resources, r.Resources[0]) }},
		{name: "update and removal", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.RemovedResources = []string{rule.Name} }},
		{name: "duplicate removal", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.RemovedResources = []string{"unknown", "unknown"} }},
		{name: "missing entry", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.RemovedResources = []string{entry.Name} }},
		{name: "invalid protobuf", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.Resources[0].Resource.Value = []byte{255} }},
		{name: "unsupported TTL", change: func(r *discoveryv3.DeltaDiscoveryResponse) { r.Resources[0].Aliases = []string{"alias"} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoader(nil, "test", slog.Default())
			applyTestBatch(t, loader, entry, rule)
			previous, _ := loader.Capture()
			versions := maps.Clone(loader.state.resources)
			response := testBatch(t, rule)
			tt.change(response)

			err := loader.apply(response)
			if err == nil {
				t.Fatal("apply(invalid batch) error = nil, want rejection")
			}

			current, _ := loader.Capture()
			if current != previous || !maps.Equal(versions, loader.state.resources) {
				t.Error("apply(rejected batch) changed the active view or accepted versions")
			}
		})
	}
}

func TestResourceBatchRebindsAndDeletesTogether(t *testing.T) {
	loader := NewLoader(nil, "test", slog.Default())
	entry, rule := testResources()
	applyTestBatch(t, loader, entry, rule)
	entry.Name, entry.GetEntry().HostName = "new-entry", "new-host"
	rule.GetModelRule().EntryRefs[0].HostName = "new-host"
	response := testBatch(t, entry, rule)
	response.RemovedResources = []string{"entry"}

	if err := loader.apply(response); err != nil {
		t.Fatalf("apply(rebind and delete old entry) error = %v", err)
	}

	view, _ := loader.Capture()
	if _, err := view.ResolveModel(8080, "host", "coding"); !errors.Is(err, ErrModelRouteNotFound) {
		t.Errorf("ResolveModel(old entry) error = %v, want no route", err)
	}

	if _, err := view.ResolveModel(8080, "new-host", "coding"); err != nil {
		t.Errorf("ResolveModel(new entry) error = %v", err)
	}

	if err := loader.apply(&discoveryv3.DeltaDiscoveryResponse{
		TypeUrl: aidataplanev1.ResourceType, Nonce: "empty", RemovedResources: []string{"new-entry", "rule"},
	}); err != nil {
		t.Fatalf("apply(remove all) error = %v", err)
	}

	empty, loaded := loader.Capture()
	if !loaded || len(empty.entries) != 0 || len(loader.state.byEntry) != 0 || len(loader.state.resources) != 0 {
		t.Errorf("Capture(remove all) = (%+v, %t), want valid empty state without stale indexes", empty, loaded)
	}
}

func TestResourceModelOrderingAndScope(t *testing.T) {
	loader := NewLoader(nil, "test", slog.Default())
	entry, exact := testResources()
	fallback := proto.CloneOf(exact)
	fallback.Name = "fallback"
	fallback.GetModelRule().Origin.Name = "first-by-name"
	fallback.GetModelRule().Matches[0].Condition = &aidataplanev1.ModelMatch_All{All: &emptypb.Empty{}}
	fallback.GetModelRule().GetTarget().ModelNameOverride = "fallback"
	exact.GetModelRule().Origin.CreationTime = timestamppb.New(time.Unix(20, 0))
	applyTestBatch(t, loader, fallback, exact, entry)
	view, _ := loader.Capture()

	got, err := view.ResolveModel(8080, "host", "coding")
	if err != nil || got.ModelNameOverride != "code-v2" {
		t.Errorf("ResolveModel(exact before older catch-all) = (%+v, %v), want code-v2", got, err)
	}

	got, err = view.ResolveModel(8080, "host", "other")
	if err != nil || got.ModelNameOverride != "fallback" {
		t.Errorf("ResolveModel(catch-all) = (%+v, %v), want fallback", got, err)
	}

	for _, key := range []hostKey{{port: 9090, name: "host"}, {port: 8080, name: "other"}} {
		if _, err := view.ResolveModel(key.port, key.name, "coding"); !errors.Is(err, ErrModelRouteNotFound) {
			t.Errorf("ResolveModel(%+v) error = %v, want no cross-entry fallback", key, err)
		}
	}

	exact.GetModelRule().Outcome = &aidataplanev1.ModelRule_Unavailable{Unavailable: &emptypb.Empty{}}
	applyTestBatch(t, loader, exact)
	view, _ = loader.Capture()
	if _, err := view.ResolveModel(8080, "host", "coding"); !errors.Is(err, ErrModelBackendUnavailable) {
		t.Errorf("ResolveModel(unavailable exact) error = %v, want refusal without catch-all fallback", err)
	}
}

func TestResourceMCPOrderingAndNoFallback(t *testing.T) {
	loader := NewLoader(nil, "test", slog.Default())
	entry, rule := testResources()
	first := &aidataplanev1.Resource{Name: "first", Payload: &aidataplanev1.Resource_McpRoute{McpRoute: &aidataplanev1.MCPRoute{
		EntryRefs: rule.GetModelRule().EntryRefs, Origin: rule.GetModelRule().Origin, Path: "/mcp",
		Outcome: &aidataplanev1.MCPRoute_Unavailable{Unavailable: &emptypb.Empty{}},
	}}}
	second := proto.CloneOf(first)
	second.Name = "second"
	second.GetMcpRoute().Origin.Name = "z"
	second.GetMcpRoute().Outcome = &aidataplanev1.MCPRoute_Target{Target: &aidataplanev1.MCPTarget{NetworkBackendName: "network", BackendPath: "/stream", TargetName: "target"}}
	applyTestBatch(t, loader, second, entry, first)
	view, _ := loader.Capture()

	if _, err := view.ResolveMCP(8080, "host", "/mcp"); !errors.Is(err, ErrMCPBackendUnavailable) {
		t.Errorf("ResolveMCP(first unavailable) error = %v, want refusal without fallback", err)
	}

	if _, err := view.ResolveMCP(8080, "host", "/mcp/"); !errors.Is(err, ErrMCPRouteNotFound) {
		t.Errorf("ResolveMCP(trailing slash) error = %v, want exact path mismatch", err)
	}

	response := testBatch(t)
	response.RemovedResources = []string{"first"}
	if err := loader.apply(response); err != nil {
		t.Fatal(err)
	}

	view, _ = loader.Capture()
	got, err := view.ResolveMCP(8080, "host", "/mcp")
	if err != nil || got.BackendPath != "/stream" || got.TargetName != "target" {
		t.Errorf("ResolveMCP(after delete winner) = (%+v, %v), want remaining configured target", got, err)
	}
}

func TestCaptureConcurrentWithResourceUpdates(t *testing.T) {
	loader := NewLoader(nil, "test", slog.Default())
	entry, rule := testResources()
	applyTestBatch(t, loader, entry, rule)
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for range 100 {
				view, loaded := loader.Capture()
				if !loaded {
					t.Error("Capture(concurrent update) returned no view")
					return
				}

				if _, err := view.ResolveModel(8080, "host", "coding"); err != nil {
					t.Errorf("ResolveModel(concurrent update) error = %v", err)
				}
			}
		})
	}

	for i := range 50 {
		rule.GetModelRule().GetTarget().ModelNameOverride = string(rune('a' + i))
		applyTestBatch(t, loader, rule)
	}

	readers.Wait()
}

func testResources() (*aidataplanev1.Resource, *aidataplanev1.Resource) {
	return &aidataplanev1.Resource{Name: "entry", Payload: &aidataplanev1.Resource_Entry{Entry: &aidataplanev1.Entry{Port: 8080, HostName: "host"}}},
		&aidataplanev1.Resource{Name: "rule", Payload: &aidataplanev1.Resource_ModelRule{ModelRule: &aidataplanev1.ModelRule{
			EntryRefs: []*aidataplanev1.EntryReference{{Port: 8080, HostName: "host"}},
			Origin:    &aidataplanev1.Origin{Namespace: "team-a", Name: "coding", CreationTime: timestamppb.New(time.Unix(1, 0))},
			Matches:   []*aidataplanev1.ModelMatch{{Condition: &aidataplanev1.ModelMatch_Exact{Exact: "coding"}}},
			Outcome: &aidataplanev1.ModelRule_Target{Target: &aidataplanev1.ModelTarget{
				BindingName: "binding", Schema: &aidataplanev1.APISchema{Name: aidataplanev1.APISchemaName_API_SCHEMA_NAME_OPENAI, Prefix: "/v1"},
				ModelNameOverride: "code-v2", NetworkBackendName: "network", TargetName: "target-v2",
			}},
		}}}
}

func testBatch(t *testing.T, resources ...*aidataplanev1.Resource) *discoveryv3.DeltaDiscoveryResponse {
	t.Helper()
	response := &discoveryv3.DeltaDiscoveryResponse{TypeUrl: aidataplanev1.ResourceType, Nonce: "nonce"}
	for _, resource := range resources {
		encoded, err := anypb.New(resource)
		if err != nil {
			t.Fatalf("anypb.New(test resource) error = %v", err)
		}

		response.Resources = append(response.Resources, &discoveryv3.Resource{Name: resource.Name, Version: cachev3.HashResource(encoded.Value), Resource: encoded})
	}

	return response
}

func applyTestBatch(t *testing.T, loader *Loader, resources ...*aidataplanev1.Resource) {
	t.Helper()
	if err := loader.apply(testBatch(t, resources...)); err != nil {
		t.Fatalf("apply(test batch) error = %v", err)
	}

	if _, loaded := loader.Capture(); !loaded {
		t.Fatal("Capture(valid batch) has no active view")
	}
}

func TestModelCandidateOrderUsesOriginAndPositions(t *testing.T) {
	_, resource := testResources()
	base := resource.GetModelRule().Origin
	older := proto.CloneOf(base)
	older.CreationTime = timestamppb.New(time.Unix(0, 0))
	otherNamespace := proto.CloneOf(base)
	otherNamespace.Namespace = "team-b"
	for _, tt := range []struct{ a, b modelCandidate }{
		{a: modelCandidate{origin: older}, b: modelCandidate{origin: base}},
		{a: modelCandidate{origin: base}, b: modelCandidate{origin: otherNamespace}},
		{a: modelCandidate{origin: base}, b: modelCandidate{origin: base, ruleIndex: 1}},
		{a: modelCandidate{origin: base}, b: modelCandidate{origin: base, matchIndex: 1}},
	} {
		if got := compareModelCandidates(tt.a, tt.b); got >= 0 {
			t.Errorf("compareModelCandidates() = %d, want negative (-want +got):\n%s", got, cmp.Diff(tt.a, tt.b, cmp.AllowUnexported(modelCandidate{})))
		}
	}
}
