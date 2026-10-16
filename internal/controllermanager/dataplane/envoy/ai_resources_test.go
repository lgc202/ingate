package envoy

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	streamv3 "github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
	"google.golang.org/protobuf/proto"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestBuildAIResources(t *testing.T) {
	config := aiFixture(t)
	before := config.NetworkBackends[0].Endpoints[0]
	target, err := aidataplane.ModelTargetName(config.Backends[0], config.NetworkBackends[0])
	if err != nil {
		t.Fatalf("ModelTargetName(golden) error = %v", err)
	}

	resources, err := buildAIResources(config, "/run/ingate/ingress.sock", "/run/ingate/egress.sock")
	if err != nil {
		t.Fatalf("buildAIResources(golden) error = %v", err)
	}

	if len(resources.listeners) != 1 || len(resources.routes) != 1 || len(resources.clusters) != 2 || len(resources.endpoints) != 0 {
		t.Fatalf("buildAIResources(golden) counts = %d/%d/%d/%d, want one egress listener, route, ingress and DNS clusters without MCP", len(resources.listeners), len(resources.routes), len(resources.clusters), len(resources.endpoints))
	}

	pipe := resources.listeners[0].Address.GetPipe()
	if pipe == nil || pipe.Path != "/run/ingate/egress.sock" || pipe.Mode != 0o600 {
		t.Errorf("buildAIResources() address = %v, want owner-only configured UDS", resources.listeners[0].Address)
	}

	routes := resources.routes[0].VirtualHosts[0].Routes
	if len(routes) != 2 {
		t.Fatalf("buildAIResources() route count = %d, want exact target and rejection", len(routes))
	}

	selected := routes[0]
	if selected.Match.Headers[0].Name != aidataplane.TargetHeader || selected.Match.Headers[0].GetStringMatch().GetExact() != target {
		t.Errorf("buildAIResources() target match = %v, want exact %q", selected.Match.Headers, target)
	}

	if selected.Match.Headers[1].Name != ":method" || selected.Match.Headers[1].GetStringMatch().GetExact() != "POST" {
		t.Errorf("buildAIResources() operation match = %v, want POST", selected.Match.Headers)
	}

	action := selected.GetRoute()
	if action.GetCluster() != clusterName(target) || action.GetClusterHeader() != "" || !action.GetAutoHostRewrite().GetValue() {
		t.Errorf("buildAIResources() action = %v, want static selected cluster and endpoint Host rewrite", action)
	}

	if action.Timeout == nil || action.Timeout.AsDuration() != 0 || action.RetryPolicy != nil {
		t.Errorf("buildAIResources() timeout/retry = %v/%v, want no whole-response timeout or retry", action.Timeout, action.RetryPolicy)
	}

	if len(selected.RequestHeadersToRemove) != 1 || selected.RequestHeadersToRemove[0] != aidataplane.TargetHeader {
		t.Errorf("buildAIResources() header removal = %v, want private target removed before upstream", selected.RequestHeadersToRemove)
	}

	if routes[1].GetDirectResponse().GetStatus() != 503 || routes[1].GetRoute() != nil {
		t.Errorf("buildAIResources() fallback = %v, want direct 503 without another backend", routes[1])
	}

	cluster := resources.clusters[1]
	if cluster.GetType() != clusterv3.Cluster_STRICT_DNS || cluster.TransportSocket != nil {
		t.Errorf("buildAIResources() cluster = %v, want Envoy DNS without unapproved TLS", cluster)
	}

	endpoint := cluster.LoadAssignment.Endpoints[0].LbEndpoints[0].GetEndpoint()
	if endpoint.Hostname != "model.internal.example:8081" {
		t.Errorf("buildAIResources() endpoint Host = %q, want backend authority", endpoint.Hostname)
	}

	if config.NetworkBackends[0].Endpoints[0] != before {
		t.Error("buildAIResources() modified source endpoint")
	}
}

func TestBuildAIResourcesRejectsAndRevokes(t *testing.T) {
	config := aiFixture(t)
	if _, err := buildAIResources(config, "/run/ingate/ingress.sock", "relative.sock"); err == nil {
		t.Error("buildAIResources(relative socket) error = nil, want rejection")
	}

	config.Listeners[0].Hosts[0].ModelRoutes[0].BackendName = new("")

	resources, err := buildAIResources(config, "/run/ingate/ingress.sock", "")
	if err != nil || len(resources.listeners) != 0 || len(resources.clusters) != 1 || len(resources.modelRoutes) != 1 {
		t.Errorf("buildAIResources(no usable model) = (%+v, %v), want Go ingress retained without an outlet", resources, err)
	}

	config = aiFixture(t)
	config.NetworkBackends[0].Endpoints[0].Address = "192.0.2.7"

	resources, err = buildAIResources(config, "/run/ingate/ingress.sock", "/run/ingate/egress.sock")
	if err != nil {
		t.Fatalf("buildAIResources(IP endpoint) error = %v", err)
	}

	if len(resources.endpoints) != 1 || resources.endpoints[0].Endpoints[0].LbEndpoints[0].GetEndpoint().Hostname != "192.0.2.7:8081" {
		t.Errorf("buildAIResources(IP endpoint) assignments = %v, want EDS and explicit authority", resources.endpoints)
	}
}

func TestPublishAIAndHTTPPreserveEachOther(t *testing.T) {
	publisher := NewPublisher("/run/ingate/ingress.sock", "/run/ingate/egress.sock")
	ai := aiFixture(t)
	httpConfig := dataplane.Config{Backends: []dataplane.Backend{{Name: "ordinary", Endpoints: []dataplane.Endpoint{{Address: "192.0.2.1", Port: 80, Weight: 1}}}}}
	var workers sync.WaitGroup
	workers.Go(func() {
		if err := publisher.Publish(t.Context(), httpConfig); err != nil {
			t.Errorf("Publish(HTTP) error = %v", err)
		}
	})
	workers.Go(func() {
		if err := publisher.PublishAI(t.Context(), ai); err != nil {
			t.Errorf("PublishAI(model) error = %v", err)
		}
	})
	workers.Wait()

	combined, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(combined) error = %v", err)
	}

	if len(combined.GetResources(resourcev3.ClusterType)) != 3 || len(combined.GetResources(resourcev3.ListenerType)) != 1 {
		t.Fatalf("combined snapshot lost HTTP or AI: clusters=%v listeners=%v", combined.GetResources(resourcev3.ClusterType), combined.GetResources(resourcev3.ListenerType))
	}

	oldRoutes := combined.GetResources(resourcev3.RouteType)[aiEgressName].(*routev3.RouteConfiguration)
	oldRoutesCopy := proto.Clone(oldRoutes)
	oldTarget := oldRoutes.VirtualHosts[0].Routes[0].Name
	ai.NetworkBackends[0].Endpoints[0].Address = "192.0.2.8"
	if err := publisher.PublishAI(t.Context(), ai); err != nil {
		t.Fatalf("PublishAI(changed target) error = %v", err)
	}

	if err := publisher.Publish(t.Context(), httpConfig); err != nil {
		t.Fatalf("Publish(repeated HTTP) error = %v", err)
	}

	changed, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(changed target) error = %v", err)
	}

	newRoutes := changed.GetResources(resourcev3.RouteType)[aiEgressName].(*routev3.RouteConfiguration)
	newTarget := newRoutes.VirtualHosts[0].Routes[0]
	if matched := newTarget.Match.Headers[0].GetStringMatch().GetExact(); matched == oldTarget || matched != newTarget.Name {
		t.Errorf("PublishAI(changed endpoint) matched target = %q, want new content-addressed target %q", matched, newTarget.Name)
	}

	if newRoutes.VirtualHosts[0].Routes[0].Name == oldTarget || changed.GetResources(resourcev3.ClusterType)[clusterName(oldTarget)] != nil {
		t.Errorf("PublishAI(changed endpoint) retained old target %q", oldTarget)
	}

	if !proto.Equal(oldRoutesCopy, combined.GetResources(resourcev3.RouteType)[aiEgressName]) {
		t.Error("PublishAI(changed target) mutated the previous snapshot")
	}

	invalid := ai
	invalid.Backends = nil
	if err := publisher.PublishAI(t.Context(), invalid); err == nil {
		t.Fatal("PublishAI(invalid) error = nil, want rejection")
	}

	afterInvalid, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil || afterInvalid != changed {
		t.Fatalf("GetSnapshot(after invalid) = (%v, %v), want previous snapshot", afterInvalid, err)
	}

	empty := aidataplane.Config{FormatVersion: aidataplane.FormatVersionV1, Listeners: []aidataplane.Listener{}, Backends: []aidataplane.Backend{}, NetworkBackends: []aidataplane.NetworkBackend{}}
	if err := publisher.PublishAI(t.Context(), empty); err != nil {
		t.Fatalf("PublishAI(revoke) error = %v", err)
	}

	revoked, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(revoked) error = %v", err)
	}

	if len(revoked.GetResources(resourcev3.ClusterType)) != 1 || revoked.GetResources(resourcev3.ClusterType)[clusterName("ordinary")] == nil || len(revoked.GetResources(resourcev3.ListenerType)) != 0 {
		t.Error("PublishAI(revoke) did not remove only AI resources")
	}
}

func TestPublishRetriesWatchAfterCacheInstallation(t *testing.T) {
	publisher := NewPublisher("/run/ingate/ingress.sock", "/run/ingate/egress.sock")
	if err := publisher.Publish(t.Context(), dataplane.Config{}); err != nil {
		t.Fatalf("Publish(initial empty) error = %v", err)
	}

	initial, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(initial empty) error = %v", err)
	}

	// 同版本、无已知资源的原生 Watch 先挂起；不接收响应，确保取消产生通知错误。
	responses := make(chan cachev3.Response)
	cancelWatch, err := publisher.cache.CreateWatch(&cachev3.Request{
		Node:        &corev3.Node{Cluster: sharedDataPlaneCluster},
		TypeUrl:     resourcev3.ClusterType,
		VersionInfo: initial.GetVersion(resourcev3.ClusterType),
	}, streamv3.NewSotwSubscription(nil, true), responses)
	if err != nil || cancelWatch == nil {
		t.Fatalf("CreateWatch(current version) error/cancel = %v/%t, want pending watch", err, cancelWatch != nil)
	}

	t.Cleanup(cancelWatch)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = publisher.PublishAI(ctx, aiFixture(t))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PublishAI(cancelled notification) error = %v, want context.Canceled", err)
	}

	installed, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil || len(installed.GetResources(resourcev3.ClusterType)) != 2 {
		t.Fatalf("SetSnapshot notification failure did not retain installed AI: snapshot=%v, error=%v", installed, err)
	}

	// 重试 HTTP 时必须保留已安装的 AI，并且相同版本也要恢复尚未完成的 Watch。
	retryCtx, retryCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer retryCancel()
	result := make(chan error, 1)
	var worker sync.WaitGroup
	worker.Go(func() { result <- publisher.Publish(retryCtx, dataplane.Config{}) })
	t.Cleanup(worker.Wait)

	select {
	case response := <-responses:
		if len(response.GetReturnedResources()) != 2 {
			t.Errorf("retry Watch resources = %v, want the installed AI ingress and target clusters", response.GetReturnedResources())
		}
	case <-retryCtx.Done():
		t.Fatal("Publish(retry unchanged snapshot) did not resume the pending native Watch")
	}

	if err := <-result; err != nil {
		t.Errorf("Publish(retry notification) error = %v, want success", err)
	}
}

func aiFixture(t *testing.T) aidataplane.Config {
	t.Helper()
	data, err := os.ReadFile("../../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(AI golden) error = %v", err)
	}

	config, err := aidataplane.Decode(data)
	if err != nil {
		t.Fatalf("Decode(AI golden) error = %v", err)
	}

	return *config
}
