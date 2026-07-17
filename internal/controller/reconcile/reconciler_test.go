package reconcile

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controllerstatus "github.com/lgc202/ingate/internal/controller/status"
	"github.com/lgc202/ingate/internal/envoy/config"
	"github.com/lgc202/ingate/internal/envoy/delivery"
	"github.com/lgc202/ingate/internal/envoy/xds"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	fakeclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestReconcileCompileErrorIsNotRetried(t *testing.T) {
	ctx := context.Background()
	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-a", Generation: 1},
		Spec:       gatewayv1.GatewaySpec{Enabled: true},
	}
	client := fakeclient.NewSimpleClientset()
	if _, err := client.GatewayV1().Gateways().Create(ctx, gateway.DeepCopy(), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create Gateway error = %v", err)
	}
	runtime := controllerstatus.NewRuntime()
	r, err := New(client, 0, nil, runtime, discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(r.queue.ShutDown)
	addToStore(t, r.factory.Gateway().V1().Gateways().Informer().GetStore(), gateway)

	r.queue.Add(queueKey)
	if !r.processNextWorkItem(ctx) {
		t.Fatal("processNextWorkItem() = false, want true")
	}
	if got := r.queue.NumRequeues(queueKey); got != 0 {
		t.Errorf("queue.NumRequeues(%q) = %d, want 0", queueKey, got)
	}

	state := runtime.Snapshot()
	if !state.Reconciled {
		t.Error("Runtime.Snapshot().Reconciled = false, want true")
	}
	if len(state.Diagnostics) == 0 || state.Diagnostics[0].Severity != config.SeverityError {
		t.Errorf("Runtime diagnostics = %#v, want an Error diagnostic", state.Diagnostics)
	}
	stored, err := client.GatewayV1().Gateways().Get(ctx, gateway.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get Gateway error = %v", err)
	}
	if got, want := stored.Status.Conditions[0].Status, metav1.ConditionFalse; got != want {
		t.Errorf("Gateway Accepted status = %q, want %q", got, want)
	}
}

func TestReconcileDeliveryErrorIsRetried(t *testing.T) {
	ctx := context.Background()
	configDelivery, err := delivery.New(
		cachev3.NewSnapshotCache(true, xds.NodeHash{}, nil),
		nil,
		delivery.Options{},
	)
	if err != nil {
		t.Fatalf("delivery.New() error = %v", err)
	}
	deliveryCtx, cancelDelivery := context.WithCancel(context.Background())
	deliveryDone := make(chan error, 1)
	go func() {
		deliveryDone <- configDelivery.Run(deliveryCtx)
	}()
	t.Cleanup(func() {
		cancelDelivery()
		select {
		case err := <-deliveryDone:
			if err != nil {
				t.Errorf("Delivery.Run() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Delivery.Run() did not stop")
		}
	})

	client := fakeclient.NewSimpleClientset()
	r, err := New(client, 0, configDelivery, controllerstatus.NewRuntime(), discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(r.queue.ShutDown)

	r.queue.Add(queueKey)
	if !r.processNextWorkItem(ctx) {
		t.Fatal("processNextWorkItem() = false, want true")
	}
	if got, want := r.queue.NumRequeues(queueKey), 1; got != want {
		t.Errorf("queue.NumRequeues(%q) = %d, want %d", queueKey, got, want)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
