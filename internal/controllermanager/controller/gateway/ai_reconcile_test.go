package gateway

import (
	"context"
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestReconcileAIStillPublishesAfterHTTPFailure(t *testing.T) {
	for _, statusFailure := range []bool{false, true} {
		fixtures := aiResourceSnapshot()
		apiClient := aiResourceClient(t, fixtures)
		cause := errors.New("HTTP output unavailable")
		var calls int
		publisher := aiPublisherStub{
			publisherFunc: func(context.Context, dataplane.Config) error {
				if !statusFailure {
					return cause
				}

				return nil
			},
			publishAI: func(context.Context, aidataplane.Config) error { calls++; return nil },
		}
		writes := interceptor.NewClient(apiClient, interceptor.Funcs{
			SubResourceUpdate: func(ctx context.Context, underlying client.Client, resource string, object client.Object, options ...client.SubResourceUpdateOption) error {
				if statusFailure {
					return cause
				}

				return underlying.SubResource(resource).Update(ctx, object, options...)
			},
		})
		reconciler := New(writes, publisher)
		loadTestCollections(t, reconciler)

		_, err := reconciler.Reconcile(t.Context(), sharedRequest())

		if !errors.Is(err, cause) || calls != 1 {
			t.Errorf("Reconcile(HTTP status failure %t) error/AI calls = %v/%d, want original error and one AI publish", statusFailure, err, calls)
		}
	}
}

func TestReconcileReturnsBothPublicationErrors(t *testing.T) {
	httpErr, aiErr := errors.New("HTTP unavailable"), errors.New("AI unavailable")
	publisher := aiPublisherStub{
		publisherFunc: func(context.Context, dataplane.Config) error { return httpErr },
		publishAI:     func(context.Context, aidataplane.Config) error { return aiErr },
	}
	reconciler := newTestReconciler(t, newFakeClient(t), publisher)

	_, err := reconciler.Reconcile(t.Context(), sharedRequest())

	if !errors.Is(err, httpErr) || !errors.Is(err, aiErr) {
		t.Errorf("Reconcile(two failures) error = %v, want both publication errors", err)
	}
}

func TestReconcileRejectsConflictingAICompilationBeforePublish(t *testing.T) {
	fixtures := aiResourceSnapshot()
	fixtures.mcpRoutes.Items[0].Spec.Path = new(aidataplane.ModelResponsesPath)
	apiClient := aiResourceClient(t, fixtures)
	var calls int
	publisher := aiPublisherStub{
		publisherFunc: func(context.Context, dataplane.Config) error { return nil },
		publishAI:     func(context.Context, aidataplane.Config) error { calls++; return nil },
	}
	reconciler := newTestReconciler(t, apiClient, publisher)

	_, err := reconciler.Reconcile(t.Context(), sharedRequest())

	if err == nil || calls != 0 {
		t.Errorf("Reconcile(conflicting AI paths) error/calls = %v/%d, want failure without publishing", err, calls)
	}
}

func aiResourceClient(t *testing.T, snapshot resourceFixtures, additional ...client.Object) client.WithWatch {
	t.Helper()

	var objects []client.Object
	for i := range snapshot.gateways.Items {
		objects = append(objects, &snapshot.gateways.Items[i])
	}

	for i := range snapshot.backends.Items {
		objects = append(objects, &snapshot.backends.Items[i])
	}

	for i := range snapshot.httpRoutes.Items {
		objects = append(objects, &snapshot.httpRoutes.Items[i])
	}

	for i := range snapshot.aiBackends.Items {
		objects = append(objects, &snapshot.aiBackends.Items[i])
	}

	for i := range snapshot.modelRoutes.Items {
		objects = append(objects, &snapshot.modelRoutes.Items[i])
	}

	for i := range snapshot.mcpRoutes.Items {
		objects = append(objects, &snapshot.mcpRoutes.Items[i])
	}

	objects = append(objects, additional...)

	return newFakeClient(t, objects...)
}
