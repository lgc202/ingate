package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

type aiPublisherStub struct {
	publisherFunc
	publishAI func(context.Context, aidataplane.Config) error
}

func TestReconcilePublishesTypedAIConfiguration(t *testing.T) {
	snapshot := aiResourceSnapshot()
	apiClient := aiResourceClient(t, snapshot)
	var published aidataplane.Config
	var calls int
	publisher := aiPublisherStub{
		publisherFunc: func(context.Context, dataplane.Config) error { return nil },
		publishAI: func(ctx context.Context, config aidataplane.Config) error {
			if ctx != t.Context() {
				t.Error("PublishAI() did not receive Reconcile context")
			}

			calls++
			published = config

			return nil
		},
	}
	r := newTestReconciler(t, apiClient, publisher)
	want, err := compileAI(snapshot.compiled())
	if err != nil {
		t.Fatalf("compileAI(expected configuration) error = %v", err)
	}

	_, err = r.Reconcile(t.Context(), sharedRequest())

	if err != nil || calls != 1 {
		t.Fatalf("Reconcile(typed AI) error/calls = %v/%d, want nil/1", err, calls)
	}

	if diff := cmp.Diff(want, published); diff != "" {
		t.Errorf("PublishAI() config mismatch (-want +got):\n%s", diff)
	}

	stored := storedAIConfigMap(t, apiClient)
	encoded, err := aidataplane.Encode(published)
	if err != nil || stored.Data[aidataplane.ConfigMapDataKey] != string(encoded) {
		t.Errorf("Reconcile() stored a different AI configuration: payload=%s, encoding error=%v", stored.Data[aidataplane.ConfigMapDataKey], err)
	}
}

func TestReconcileAIOutputsFailIndependently(t *testing.T) {
	for _, tt := range []struct {
		name          string
		failXDS       bool
		failConfigMap bool
	}{
		{name: "xDS failure", failXDS: true},
		{name: "ConfigMap failure", failConfigMap: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := aiResourceSnapshot()
			apiClient := aiResourceClient(t, snapshot)
			cause := errors.New("publication unavailable")
			var xdsCalls, configMapCalls int
			writes := interceptor.NewClient(apiClient, interceptor.Funcs{
				Create: func(
					ctx context.Context,
					underlying client.WithWatch,
					object client.Object,
					options ...client.CreateOption,
				) error {
					configMapCalls++
					if tt.failConfigMap {
						return cause
					}

					return underlying.Create(ctx, object, options...)
				},
			})
			publisher := aiPublisherStub{
				publisherFunc: func(context.Context, dataplane.Config) error { return nil },
				publishAI: func(context.Context, aidataplane.Config) error {
					xdsCalls++
					if tt.failXDS {
						return cause
					}

					return nil
				},
			}
			r := New(writes, apiClient, publisher, "control-plane")
			loadTestCollections(t, r)

			_, err := r.Reconcile(t.Context(), sharedRequest())

			if !errors.Is(err, cause) || xdsCalls != 1 || configMapCalls != 1 {
				t.Errorf("Reconcile(%s) = (error %v, xDS %d, ConfigMap %d), want wrapped failure and both outputs attempted", tt.name, err, xdsCalls, configMapCalls)
			}

			if tt.failXDS && storedAIConfigMap(t, apiClient).Data[aidataplane.ConfigMapDataKey] == "" {
				t.Error("Reconcile(xDS failure) did not store complete AI payload")
			}
		})
	}
}

func (p aiPublisherStub) PublishAI(ctx context.Context, config aidataplane.Config) error {
	return p.publishAI(ctx, config)
}
