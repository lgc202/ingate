package route

import (
	"reflect"
	"sort"
	"testing"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"

	controllerindex "github.com/lgc202/ingate/internal/controlplane/controller/index"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
)

func TestRouteControllerEnqueuesAffectedGateway(t *testing.T) {
	idx := controllerindex.New()
	queue := &recordingQueue{}
	controller := &Controller{index: idx, queue: queue}

	idx.UpsertGateway(gateway("public-edge"))

	controller.onAdd(route("catalog-route", []string{"public-edge"}, []string{"catalog-backend"}))
	if got, want := queue.Keys(), []string{"public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected keys after add: got %v, want %v", got, want)
	}

	queue.Reset()
	controller.onUpdate(
		route("catalog-route", []string{"public-edge"}, []string{"catalog-backend"}),
		route("catalog-route", []string{"private-edge"}, []string{"catalog-backend"}),
	)
	if got, want := queue.Keys(), []string{"private-edge", "public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected keys after update: got %v, want %v", got, want)
	}

	queue.Reset()
	controller.onDelete(route("catalog-route", []string{"private-edge"}, []string{"catalog-backend"}))
	if got, want := queue.Keys(), []string{"private-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected keys after delete: got %v, want %v", got, want)
	}
}

type recordingQueue struct {
	keys []shared.ObjectKey
}

func (q *recordingQueue) Enqueue(key shared.ObjectKey)  { q.keys = append(q.keys, key) }
func (q *recordingQueue) Requeue(key shared.ObjectKey)  { q.keys = append(q.keys, key) }
func (q *recordingQueue) Get() (shared.ObjectKey, bool) { return shared.ObjectKey{}, true }
func (q *recordingQueue) Done(shared.ObjectKey)         {}
func (q *recordingQueue) Forget(shared.ObjectKey)       {}
func (q *recordingQueue) ShutDown()                     {}
func (q *recordingQueue) Reset()                        { q.keys = nil }
func (q *recordingQueue) Keys() []string {
	items := make([]string, 0, len(q.keys))
	for _, key := range q.keys {
		items = append(items, key.String())
	}
	sort.Strings(items)
	return items
}

func gateway(name string) *gatewayv1alpha1.Gateway {
	obj := &gatewayv1alpha1.Gateway{}
	obj.Name = name
	return obj
}

func route(name string, parents, backends []string) *gatewayv1alpha1.Route {
	obj := &gatewayv1alpha1.Route{}
	obj.Name = name
	for _, parent := range parents {
		obj.Spec.ParentRefs = append(obj.Spec.ParentRefs, gatewayv1alpha1.ParentReference{Name: parent})
	}
	obj.Spec.Rules = []gatewayv1alpha1.RouteRule{{}}
	for _, backend := range backends {
		obj.Spec.Rules[0].BackendRefs = append(obj.Spec.Rules[0].BackendRefs, gatewayv1alpha1.BackendRef{Name: backend})
	}
	return obj
}
