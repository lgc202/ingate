package envoy

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	streamv3 "github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
)

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
