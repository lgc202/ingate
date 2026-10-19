package envoy

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestAITranslationReusesUnaffectedEntriesAndTargets(t *testing.T) {
	config := aiFixture(t)
	config.Listeners = append(config.Listeners, aidataplane.Listener{Port: 9090, Hosts: config.Listeners[0].Hosts})
	var empty aiTranslation

	first := compileTestAI(t, &empty, config)
	again := compileTestAI(t, &first, config)
	key := entryKey{routeName: "route/8080", hostName: config.Listeners[0].Hosts[0].Name}
	other := entryKey{routeName: "route/9090", hostName: key.hostName}
	model, mcp := first.egressTargets[0], first.egressTargets[1]

	// 在 Snapshot 的哈希去重之前验证复用，不能只验证最终输出内容相等。
	if again.entries[key].routes[0] != first.entries[key].routes[0] ||
		again.targets[model].cluster != first.targets[model].cluster ||
		again.targets[mcp].route != first.targets[mcp].route ||
		again.ingressCluster != first.ingressCluster || again.egressListener != first.egressListener || again.egressRoutes != first.egressRoutes {
		t.Error("compile(unchanged AI) rebuilt cached ingress, targets or UDS resources")
	}

	config.Listeners[0].Hosts[0].ModelRoutes[0].Headers[0].Value = "new-model"
	config.Listeners[0].Hosts[0].MCPRoutes[0].BackendPath = "/new-upstream"

	matching := compileTestAI(t, &again, config)

	if matching.entries[key].routes[0] != again.entries[key].routes[0] || matching.egressRoutes != again.egressRoutes {
		t.Error("compile(Go-only matching or upstream path) rebuilt Envoy ingress or egress RDS")
	}

	// 拆开共享 Host 切片，后续路径变化只作用于 8080。
	config.Listeners[0].Hosts = slices.Clone(config.Listeners[0].Hosts)
	config.Listeners[0].Hosts[0].MCPRoutes = slices.Clone(config.Listeners[0].Hosts[0].MCPRoutes)
	config.Listeners[0].Hosts[0].MCPRoutes[0].Path = "/v2/mcp"

	path := compileTestAI(t, &matching, config)

	if path.entries[key].routes[0] == matching.entries[key].routes[0] ||
		path.entries[other].routes[0] != matching.entries[other].routes[0] || path.egressRoutes != matching.egressRoutes {
		t.Error("compile(changed 8080 MCP path) did not rebuild only the affected ingress entry")
	}

	if got := first.entries[key].paths[0]; got != "/mcp" {
		t.Errorf("compile(changed caller path) changed previous entry path = %q, want /mcp", got)
	}

	*config.Backends[0].ModelNameOverride = "new-upstream-model"

	binding := compileTestAI(t, &path, config)
	newModel := binding.egressTargets[0]

	if newModel == model || binding.targets[newModel].cluster == path.targets[model].cluster || binding.targets[model].cluster != nil {
		t.Error("compile(changed model binding) did not replace and withdraw the old model target")
	}

	if binding.targets[mcp].cluster != path.targets[mcp].cluster || binding.egressListener != path.egressListener ||
		binding.ingressCluster != path.ingressCluster || binding.entries[key].routes[0] != path.entries[key].routes[0] {
		t.Error("compile(changed model binding) rebuilt unrelated MCP, UDS or ingress resources")
	}
}

func TestAITranslationUpdatesOnlyDependentTargets(t *testing.T) {
	config := aiFixture(t)
	second := config.Backends[0]
	second.Name = "team-a/second-model"
	config.Backends = append(config.Backends, second)
	route := config.Listeners[0].Hosts[0].ModelRoutes[0]
	route.Name = "team-a/second-route"
	route.BackendName = new(second.Name)
	config.Listeners[0].Hosts[0].ModelRoutes = append(config.Listeners[0].Hosts[0].ModelRoutes, route)
	config.NetworkBackends[0].Endpoints = []aidataplane.Endpoint{
		{Address: "192.0.2.1", Port: 8081, Weight: 1},
		{Address: "192.0.2.2", Port: 8081, Weight: 2},
	}
	var empty aiTranslation

	first := compileTestAI(t, &empty, config)
	mcp := first.egressTargets[2]
	oldModel, oldSecond := first.egressTargets[0], first.egressTargets[1]

	// 目标哈希与端点转换都规范化顺序，单纯重排不应重建消息。
	slices.Reverse(config.NetworkBackends[0].Endpoints)

	reordered := compileTestAI(t, &first, config)

	if reordered.targets[oldModel].cluster != first.targets[oldModel].cluster || reordered.egressRoutes != first.egressRoutes {
		t.Error("compile(reordered endpoints) rebuilt normalized target or egress RDS")
	}

	config.NetworkBackends[0].Endpoints[0].Address = "192.0.2.3"

	changed := compileTestAI(t, &reordered, config)

	if changed.egressTargets[0] == oldModel || changed.egressTargets[1] == oldSecond ||
		changed.targets[oldModel].cluster != nil || changed.targets[oldSecond].cluster != nil || len(changed.resources.endpoints) != 2 {
		t.Error("compile(shared network update) failed to replace both dependent model targets and their EDS")
	}

	if changed.targets[mcp].cluster != reordered.targets[mcp].cluster ||
		changed.targets[mcp].route != reordered.targets[mcp].route || changed.ingressCluster != reordered.ingressCluster ||
		changed.egressListener != reordered.egressListener {
		t.Error("compile(shared network update) rebuilt unrelated MCP or UDS resources")
	}

	if first.targets[oldModel].endpoints.Endpoints[0].LbEndpoints[1].GetEndpoint().GetAddress().GetSocketAddress().Address != "192.0.2.2" {
		t.Error("compile(changed caller endpoints) mutated the previous assignment")
	}

	config.NetworkBackends[0].TLS = &aidataplane.BackendTLSSettings{SNI: "model.example.com", WellKnownCACertificates: "System"}

	tls := compileTestAI(t, &changed, config)

	if tls.egressTargets[0] == changed.egressTargets[0] || tls.targets[tls.egressTargets[0]].cluster.TransportSocket == nil ||
		tls.targets[mcp].cluster != changed.targets[mcp].cluster {
		t.Error("compile(TLS identity change) did not rebuild only the dependent TLS targets")
	}
}

func TestAITranslationWithdrawsUnusedResources(t *testing.T) {
	config := aiFixture(t)
	duplicate := config.Listeners[0].Hosts[0].MCPRoutes[0]
	duplicate.Name = "team-a/duplicate-path"
	config.Listeners[0].Hosts[0].MCPRoutes = append(config.Listeners[0].Hosts[0].MCPRoutes, duplicate)
	var empty aiTranslation

	first := compileTestAI(t, &empty, config)
	key := entryKey{routeName: "route/8080", hostName: config.Listeners[0].Hosts[0].Name}
	mcp := first.egressTargets[1]

	if len(first.entries[key].routes) != 3 || len(first.targets) != 2 {
		t.Fatal("compile(duplicate MCP path and backend) did not deduplicate ingress and target")
	}

	config.Listeners[0].Hosts[0].ModelRoutes[0].BackendName = new("")
	config.NetworkBackends[0].Endpoints[0].Address = "unused.example.com"

	unused := compileTestAI(t, &first, config)

	if len(unused.targets) != 1 || unused.targets[mcp].cluster != first.targets[mcp].cluster ||
		unused.entries[key].routes[0] != first.entries[key].routes[0] {
		t.Error("compile(unavailable model) failed to withdraw its target while retaining ingress and MCP")
	}

	config.Listeners[0].Hosts[0].ModelRoutes = []aidataplane.ModelRoute{}

	disabled := compileTestAI(t, &unused, config)

	if len(disabled.entries[key].routes) != 1 || disabled.entries[key].routes[0].Match.GetPath() != "/mcp" ||
		disabled.egressRoutes != unused.egressRoutes {
		t.Error("compile(removed model routes) retained model operations or rebuilt the unchanged MCP outlet")
	}

	for i := range config.Listeners[0].Hosts[0].MCPRoutes {
		config.Listeners[0].Hosts[0].MCPRoutes[i].BackendName = new("")
	}

	unavailable := compileTestAI(t, &disabled, config)

	if len(unavailable.targets) != 0 || unavailable.egressRoutes != nil || unavailable.egressListener != nil ||
		unavailable.ingressCluster != unused.ingressCluster || len(unavailable.entries) != 1 {
		t.Error("compile(all targets unavailable) retained egress or removed the rejection ingress")
	}

	config.Listeners = []aidataplane.Listener{}

	cleared := compileTestAI(t, &unavailable, config)

	if len(cleared.entries) != 0 || len(cleared.targets) != 0 || cleared.ingressCluster != nil ||
		cleared.egressListener != nil || cleared.egressRoutes != nil || len(cleared.egressTargets) != 0 {
		t.Error("compile(empty listeners) retained obsolete translation entries")
	}

	recreated := compileTestAI(t, &cleared, aiFixture(t))

	if recreated.targets[mcp].cluster == first.targets[mcp].cluster || recreated.ingressCluster == first.ingressCluster {
		t.Error("compile(recreated AI) recovered obsolete historical cache entries")
	}
}

func TestAITranslationInvalidatesOnlyChangedSocket(t *testing.T) {
	config := aiFixture(t)
	var empty aiTranslation

	first := compileTestAI(t, &empty, config)

	egress, err := first.compile(config, first.ingressSocket, "/run/ingate/new-egress.sock")
	if err != nil {
		t.Fatalf("compile(changed egress socket) error = %v", err)
	}

	model := first.egressTargets[0]
	if egress.egressListener == first.egressListener || egress.egressListener.Address.GetPipe().Path != egress.egressSocket ||
		egress.egressRoutes != first.egressRoutes || egress.ingressCluster != first.ingressCluster ||
		egress.targets[model].cluster != first.targets[model].cluster {
		t.Error("compile(changed egress socket) did not rebuild only the egress Listener")
	}

	ingress, err := egress.compile(config, "/run/ingate/new-ingress.sock", egress.egressSocket)
	if err != nil {
		t.Fatalf("compile(changed ingress socket) error = %v", err)
	}

	if ingress.ingressCluster == egress.ingressCluster || ingress.egressListener != egress.egressListener ||
		ingress.egressRoutes != egress.egressRoutes || ingress.targets[model].cluster != egress.targets[model].cluster {
		t.Error("compile(changed ingress socket) did not rebuild only the ingress Cluster")
	}

	if _, err := ingress.compile(config, ingress.ingressSocket, ingress.ingressSocket); err == nil {
		t.Error("compile(same ingress and egress socket after cache hit) error = nil, want rejection")
	}
}

func TestPublishAIRetainsTranslationAfterRejectedCandidate(t *testing.T) {
	config := aiFixture(t)
	publisher := NewPublisher("/run/ingate/ingress.sock", "/run/ingate/egress.sock")
	if err := publisher.PublishAI(t.Context(), config); err != nil {
		t.Fatalf("PublishAI(initial) error = %v", err)
	}

	previous := publisher.ai
	model := previous.egressTargets[0]
	before := proto.Clone(previous.egressRoutes)

	// 入口候选已转换，但出口套接字校验失败，不能安装局部结果。
	config.Listeners[0].Hosts[0].MCPRoutes[0].Path = "/new-mcp"
	publisher.aiEgressSocket = "relative.sock"

	if err := publisher.PublishAI(t.Context(), config); err == nil {
		t.Fatal("PublishAI(invalid egress socket) error = nil, want rejection")
	}

	if publisher.ai.egressRoutes != previous.egressRoutes || publisher.ai.targets[model].cluster != previous.targets[model].cluster {
		t.Error("PublishAI(rejected socket) replaced cached AI resources")
	}

	key := entryKey{routeName: "route/8080", hostName: config.Listeners[0].Hosts[0].Name}
	if publisher.ai.entries[key].routes[0] != previous.entries[key].routes[0] {
		t.Error("PublishAI(rejected socket) partially installed a new ingress entry")
	}

	publisher.aiEgressSocket = "/run/ingate/egress.sock"
	config.Listeners[0].Hosts[0].MCPRoutes[0].Path = aidataplane.ModelResponsesPath

	if err := publisher.PublishAI(t.Context(), config); err == nil {
		t.Fatal("PublishAI(model and MCP path conflict) error = nil, want rejection")
	}

	config.Listeners[0].Hosts[0].MCPRoutes[0].Path = "/mcp"
	config.NetworkBackends[0].Endpoints[0].Port = 70000

	if err := publisher.PublishAI(t.Context(), config); err == nil {
		t.Fatal("PublishAI(invalid network after cache hit) error = nil, want validation")
	}

	if err := publisher.Publish(t.Context(), dataplane.Config{}); err != nil {
		t.Fatalf("Publish(HTTP after rejected AI) error = %v", err)
	}

	if !proto.Equal(publisher.ai.egressRoutes, before) || publisher.ai.targets[model].cluster != previous.targets[model].cluster {
		t.Error("Publish(HTTP after rejected AI) lost the last valid AI translation")
	}
}

func compileTestAI(t *testing.T, previous *aiTranslation, config aidataplane.Config) aiTranslation {
	t.Helper()
	translated, err := previous.compile(config, "/run/ingate/ingress.sock", "/run/ingate/egress.sock")
	if err != nil {
		t.Fatalf("compile(AI configuration) error = %v", err)
	}

	want, err := buildAIResourcesReference(config, "/run/ingate/ingress.sock", "/run/ingate/egress.sock")
	if err != nil {
		t.Fatalf("buildAIResources(reference) error = %v", err)
	}

	if diff := cmp.Diff(want, translated.resources, cmp.AllowUnexported(resourceSet{}, entryKey{}), protocmp.Transform()); diff != "" {
		t.Fatalf("incremental AI translation mismatch (-want +got):\n%s", diff)
	}

	return translated
}
