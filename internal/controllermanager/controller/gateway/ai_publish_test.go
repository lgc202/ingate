package gateway

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
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
}

func (p aiPublisherStub) PublishAI(ctx context.Context, config aidataplane.Config, _ []*aidataplanev1.Resource) error {
	return p.publishAI(ctx, config)
}
