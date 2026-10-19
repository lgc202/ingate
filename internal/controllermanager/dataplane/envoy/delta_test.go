package envoy

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	streamv3 "github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

func TestDeltaADSUpdatesDeletesAndResumes(t *testing.T) {
	config := twoPortConfig()
	publisher := NewPublisher("", "")
	initial := publishTestSnapshot(t, publisher, config)
	client := newTestADSClient(t, publisher, logr.Discard())
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	streamCtx, closeStream := context.WithCancel(ctx)
	defer closeStream()
	stream, err := client.DeltaAggregatedResources(streamCtx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(initial) error = %v", err)
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{
		Node:                   &corev3.Node{Id: "delta-envoy", Cluster: sharedDataPlaneCluster},
		TypeUrl:                resourcev3.RouteType,
		ResourceNamesSubscribe: []string{"*"},
	})
	first := receiveTestDelta(t, stream)
	versions := maps.Clone(initial.GetVersionMap(resourcev3.RouteType))
	if len(first.Resources) != 2 || len(first.RemovedResources) != 0 {
		t.Fatalf("Delta ADS(initial RDS) resources/removals = %d/%v, want 2/[]", len(first.Resources), first.RemovedResources)
	}

	for _, resource := range first.Resources {
		if resource.Version == "" || resource.Version != versions[resource.Name] || resource.Resource == nil {
			t.Errorf("Delta ADS(initial RDS %q) version/payload = %q/%v, want native version and payload", resource.Name, resource.Version, resource.Resource)
		}
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{TypeUrl: resourcev3.RouteType, ResponseNonce: first.Nonce})
	config.Listeners[0].Hosts[0].Routes[0].Path = "/v2/orders"
	changed := publishTestSnapshot(t, publisher, config)

	update := receiveTestDelta(t, stream)
	if len(update.Resources) != 1 || update.Resources[0].Name != "route/8080" || len(update.RemovedResources) != 0 {
		t.Fatalf("Delta ADS(changed 8080) = %v, want only route/8080 and no removal", update)
	}

	resource := update.Resources[0]
	if resource.Version == versions[resource.Name] || resource.Version != changed.GetVersionMap(resourcev3.RouteType)[resource.Name] {
		t.Error("Delta ADS(changed 8080) did not use the changed native resource version")
	}

	var routes routev3.RouteConfiguration
	if err := resource.Resource.UnmarshalTo(&routes); err != nil {
		t.Fatalf("UnmarshalTo(changed RDS) error = %v", err)
	}

	if got := routes.VirtualHosts[0].Routes[0].Match.GetPathSeparatedPrefix(); got != "/v2/orders" {
		t.Errorf("Delta ADS(changed RDS) path = %q, want /v2/orders", got)
	}

	versions[resource.Name] = resource.Version
	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{TypeUrl: resourcev3.RouteType, ResponseNonce: update.Nonce})

	// 同内容发布不能排入多余响应；随后修改另一个端口，下一包必须只含 9090。
	publishTestSnapshot(t, publisher, config)
	config.Listeners[1].Hosts[0].Routes[0].Path = "/v2/operations"
	publishTestSnapshot(t, publisher, config)

	other := receiveTestDelta(t, stream)
	if len(other.Resources) != 1 || other.Resources[0].Name != "route/9090" || len(other.RemovedResources) != 0 {
		t.Fatalf("Delta ADS(unchanged then changed 9090) = %v, want only route/9090", other)
	}

	versions[other.Resources[0].Name] = other.Resources[0].Version
	closeStream()

	// 断线期间删除 8080 并修改 9090；重连须依据客户端实际持有的版本对账。
	config.Listeners = config.Listeners[1:]
	config.Backends = config.Backends[1:]
	config.Listeners[0].Hosts[0].Routes[0].Path = "/v3/operations"
	current := publishTestSnapshot(t, publisher, config)
	resumed, err := client.DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(reconnect) error = %v", err)
	}

	sendTestDelta(t, resumed, &discoveryv3.DeltaDiscoveryRequest{
		Node:                    &corev3.Node{Id: "delta-envoy", Cluster: sharedDataPlaneCluster},
		TypeUrl:                 resourcev3.RouteType,
		ResourceNamesSubscribe:  []string{"*"},
		InitialResourceVersions: versions,
	})
	catchup := receiveTestDelta(t, resumed)
	if len(catchup.Resources) != 1 || catchup.Resources[0].Name != "route/9090" ||
		catchup.Resources[0].Version != current.GetVersionMap(resourcev3.RouteType)["route/9090"] ||
		!slices.Equal(catchup.RemovedResources, []string{"route/8080"}) {
		t.Fatalf("Delta ADS(reconnect) = %v, want changed 9090 and removed 8080", catchup)
	}

	sendTestDelta(t, resumed, &discoveryv3.DeltaDiscoveryRequest{TypeUrl: resourcev3.RouteType, ResponseNonce: catchup.Nonce})
	publishTestSnapshot(t, publisher, dataplane.Config{})

	deleted := receiveTestDelta(t, resumed)
	if len(deleted.Resources) != 0 || !slices.Equal(deleted.RemovedResources, []string{"route/9090"}) {
		t.Errorf("Delta ADS(withdraw all) = %v, want only removed route/9090", deleted)
	}
}

func TestDeltaADSRespectsNamedSubscriptions(t *testing.T) {
	config := twoPortConfig()
	publisher := NewPublisher("", "")
	publishTestSnapshot(t, publisher, config)
	client := newTestADSClient(t, publisher, logr.Discard())
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	stream, err := client.DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(named EDS) error = %v", err)
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{
		Node:                   &corev3.Node{Id: "delta-envoy", Cluster: sharedDataPlaneCluster},
		TypeUrl:                resourcev3.EndpointType,
		ResourceNamesSubscribe: []string{"backend/orders"},
	})
	first := receiveTestDelta(t, stream)
	if len(first.Resources) != 1 || first.Resources[0].Name != "backend/orders" {
		t.Fatalf("Delta ADS(named EDS) = %v, want only backend/orders", first)
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{TypeUrl: resourcev3.EndpointType, ResponseNonce: first.Nonce})
	// 未订阅的 operations 变化不能发包；随后 orders 变化才应产生下一包。
	config.Backends[1].Endpoints[0].Address = "192.0.2.10"
	publishTestSnapshot(t, publisher, config)
	config.Backends[0].Endpoints[0].Address = "192.0.2.11"
	changed := publishTestSnapshot(t, publisher, config)

	update := receiveTestDelta(t, stream)
	if len(update.Resources) != 1 || update.Resources[0].Name != "backend/orders" ||
		update.Resources[0].Version != changed.GetVersionMap(resourcev3.EndpointType)["backend/orders"] || len(update.RemovedResources) != 0 {
		t.Fatalf("Delta ADS(only subscribed endpoint changed) = %v, want current backend/orders", update)
	}

	// 即使 nonce 过期，订阅增减仍必须生效，回调不能把它当成非法请求拦截。
	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{
		TypeUrl:                  resourcev3.EndpointType,
		ResponseNonce:            "stale",
		ResourceNamesSubscribe:   []string{"backend/operations"},
		ResourceNamesUnsubscribe: []string{"backend/orders"},
	})
	switched := receiveTestDelta(t, stream)
	if len(switched.Resources) != 1 || switched.Resources[0].Name != "backend/operations" || len(switched.RemovedResources) != 0 {
		t.Errorf("Delta ADS(switched subscription) = %v, want only operations, not an orders deletion", switched)
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{TypeUrl: resourcev3.EndpointType, ResponseNonce: switched.Nonce})
	config.Backends[0].Endpoints[0].Address = "192.0.2.12"
	publishTestSnapshot(t, publisher, config)
	config.Backends[1].Endpoints[0].Address = "192.0.2.13"
	current := publishTestSnapshot(t, publisher, config)

	afterUnsubscribe := receiveTestDelta(t, stream)
	if len(afterUnsubscribe.Resources) != 1 || afterUnsubscribe.Resources[0].Name != "backend/operations" ||
		afterUnsubscribe.Resources[0].Version != current.GetVersionMap(resourcev3.EndpointType)["backend/operations"] {
		t.Errorf("Delta ADS(after orders unsubscribe) = %v, want only the new operations version", afterUnsubscribe)
	}
}

func TestDeltaADSReconnectSkipsKnownResources(t *testing.T) {
	publisher := NewPublisher("", "")
	initial := publishTestSnapshot(t, publisher, twoPortConfig())
	client := newTestADSClient(t, publisher, logr.Discard())
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	stream, err := client.DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(known versions) error = %v", err)
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{
		Node:                    &corev3.Node{Id: "resumed-envoy", Cluster: sharedDataPlaneCluster},
		TypeUrl:                 resourcev3.RouteType,
		ResourceNamesSubscribe:  []string{"*"},
		InitialResourceVersions: initial.GetVersionMap(resourcev3.RouteType),
	})
	response := receiveTestDelta(t, stream)
	if len(response.Resources) != 0 || len(response.RemovedResources) != 0 {
		t.Errorf("Delta ADS(all versions known) = %v, want empty initial response without resending either RDS", response)
	}
}

func TestDeltaADSEmptySnapshotAndSotWDisabled(t *testing.T) {
	publisher := NewPublisher("", "")
	publishTestSnapshot(t, publisher, dataplane.Config{})
	client := newTestADSClient(t, publisher, logr.Discard())
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	stream, err := client.DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources(empty) error = %v", err)
	}

	sendTestDelta(t, stream, &discoveryv3.DeltaDiscoveryRequest{
		Node:                   &corev3.Node{Id: "delta-envoy", Cluster: sharedDataPlaneCluster},
		TypeUrl:                resourcev3.ListenerType,
		ResourceNamesSubscribe: []string{"*"},
	})
	empty := receiveTestDelta(t, stream)
	if len(empty.Resources) != 0 || len(empty.RemovedResources) != 0 {
		t.Errorf("Delta ADS(empty snapshot) = %v, want empty initial response", empty)
	}

	legacy, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("StreamAggregatedResources(disabled) open error = %v", err)
	}

	if _, err := legacy.Recv(); status.Code(err) != codes.Unimplemented {
		t.Errorf("StreamAggregatedResources(disabled) error = %v, want Unimplemented", err)
	}
}

func TestDeltaNotificationFailureRetainsDesiredAndRetries(t *testing.T) {
	config := twoPortConfig()
	publisher := NewPublisher("", "")
	initial := publishTestSnapshot(t, publisher, config)
	responses := make(chan cachev3.DeltaResponse)
	subscription := streamv3.NewDeltaSubscription([]string{"*"}, nil, initial.GetVersionMap(resourcev3.EndpointType), true)
	cancelWatch, err := publisher.cache.CreateDeltaWatch(&cachev3.DeltaRequest{
		Node:          &corev3.Node{Cluster: sharedDataPlaneCluster},
		TypeUrl:       resourcev3.EndpointType,
		ResponseNonce: "accepted",
	}, subscription, responses)
	if err != nil || cancelWatch == nil {
		t.Fatalf("CreateDeltaWatch(current EDS) error/cancel = %v/%t, want pending watch", err, cancelWatch != nil)
	}

	t.Cleanup(cancelWatch)
	config.Backends[0].Endpoints[0].Address = "192.0.2.3"
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = publisher.Publish(ctx, config)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish(cancelled Delta notification) error = %v, want context.Canceled", err)
	}

	desired := publisher.http.backends["orders"]
	if desired.source.Endpoints[0].Address != "192.0.2.3" {
		t.Fatal("Publish(cancelled Delta notification) rolled back the new desired translation")
	}

	retryCtx, retryCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer retryCancel()
	result := make(chan error, 1)
	var worker sync.WaitGroup
	worker.Go(func() { result <- publisher.Publish(retryCtx, config) })
	t.Cleanup(worker.Wait)

	select {
	case response := <-responses:
		message, err := response.GetDeltaDiscoveryResponse()
		if err != nil {
			t.Fatalf("GetDeltaDiscoveryResponse(retry) error = %v", err)
		}

		if len(message.Resources) != 1 || message.Resources[0].Name != "backend/orders" || len(message.RemovedResources) != 0 {
			t.Errorf("Delta notification retry = %v, want only changed backend/orders", message)
		}
	case <-retryCtx.Done():
		t.Fatal("Publish(identical retry) did not resume the pending Delta Watch")
	}

	if err := <-result; err != nil {
		t.Fatalf("Publish(Delta retry) error = %v", err)
	}

	if publisher.http.backends["orders"].cluster != desired.cluster || publisher.http.backends["orders"].endpoints != desired.endpoints {
		t.Error("Publish(Delta retry) rebuilt unchanged Backend translation")
	}
}

func sendTestDelta(
	t *testing.T,
	stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesClient,
	request *discoveryv3.DeltaDiscoveryRequest,
) {
	t.Helper()
	if err := stream.Send(request); err != nil {
		t.Fatalf("Delta ADS Send(%s) error = %v", request.TypeUrl, err)
	}
}

func receiveTestDelta(
	t *testing.T,
	stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesClient,
) *discoveryv3.DeltaDiscoveryResponse {
	t.Helper()
	response, err := stream.Recv()
	if err != nil {
		t.Fatalf("Delta ADS Recv() error = %v", err)
	}

	if response.TypeUrl == "" || response.Nonce == "" {
		t.Fatalf("Delta ADS Recv() type/nonce = %q/%q, want nonempty", response.TypeUrl, response.Nonce)
	}

	return response
}
