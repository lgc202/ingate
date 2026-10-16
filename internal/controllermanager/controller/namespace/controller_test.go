package namespace

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
	metadatafake "k8s.io/client-go/metadata/fake"
	clienttesting "k8s.io/client-go/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/lgc202/ingate/internal/controllermanager/controller/namespace/deletion"
)

func TestReconcileCompletesDeletion(t *testing.T) {
	r, kubeClient, _ := reconcileFixture(t, terminatingNamespace())
	result, err := r.Reconcile(t.Context(), namespaceRequest())
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("Reconcile(empty namespace) = %v, %v, want no retry and nil", result, err)
	}

	namespace, err := kubeClient.CoreV1().Namespaces().Get(t.Context(), "team-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(team-a) error = %v", err)
	}

	if !slices.Equal(namespace.Spec.Finalizers, []corev1.FinalizerName{"example.com/retain"}) {
		t.Errorf("Reconcile(empty namespace) finalizers = %v, want custom finalizer retained", namespace.Spec.Finalizers)
	}
}

func TestReconcileIgnoresInactiveNamespaces(t *testing.T) {
	active := terminatingNamespace()
	active.DeletionTimestamp = nil
	finalized := terminatingNamespace()
	finalized.Spec.Finalizers = nil
	tests := []struct {
		name      string
		namespace *corev1.Namespace
	}{
		{name: "missing"},
		{name: "active", namespace: active},
		{name: "finalized", namespace: finalized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, kubeClient, metadataClient := reconcileFixture(t, tt.namespace)
			result, err := r.Reconcile(t.Context(), namespaceRequest())
			if err != nil || result.RequeueAfter != 0 {
				t.Errorf("Reconcile(%s) = %v, %v, want no retry and nil", tt.name, result, err)
			}

			if len(kubeClient.Actions()) != 0 || len(metadataClient.Actions()) != 0 {
				t.Errorf("Reconcile(%s) accesses deletion clients, want skipped cleanup", tt.name)
			}
		})
	}
}

func TestReconcileDeletionGracePeriod(t *testing.T) {
	namespace := terminatingNamespace()
	now := metav1.Now()
	namespace.DeletionTimestamp = &now
	r, kubeClient, metadataClient := reconcileFixture(t, namespace)
	result, err := r.Reconcile(t.Context(), namespaceRequest())
	if err != nil || result.RequeueAfter <= 0 || result.RequeueAfter > namespaceDeletionGracePeriod {
		t.Errorf("Reconcile(recent deletion) = %v, %v, want delayed retry within %v", result, err, namespaceDeletionGracePeriod)
	}

	if len(kubeClient.Actions()) != 0 || len(metadataClient.Actions()) != 0 {
		t.Error("Reconcile(recent deletion) accesses deletion clients before grace period ends")
	}
}

func TestReconcileWaitsWithoutFailure(t *testing.T) {
	resource := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}
	item := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Widget"},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "edge",
			Namespace:  "team-a",
			Finalizers: []string{"example.com/content"},
		},
	}
	r, kubeClient, metadataClient := reconcileFixture(t, terminatingNamespace(), item)
	metadataClient.PrependReactor("delete-collection", "widgets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, &metav1.Status{Status: metav1.StatusSuccess}, nil
	})
	discoverResources := func(context.Context) ([]*metav1.APIResourceList, error) {
		return []*metav1.APIResourceList{{
			GroupVersion: resource.GroupVersion().String(),
			APIResources: []metav1.APIResource{{
				Name:       resource.Resource,
				Namespaced: true,
				Verbs:      metav1.Verbs{"list", "delete", "deletecollection"},
			}},
		}}, nil
	}
	r.deleter = deletion.New(kubeClient.CoreV1().Namespaces(), metadataClient, discoverResources)

	result, err := r.Reconcile(t.Context(), namespaceRequest())
	if err != nil || result.RequeueAfter != 8*time.Second {
		t.Errorf("Reconcile(content finalizer) = %v, %v, want retry in 8s without error", result, err)
	}

	for _, action := range kubeClient.Actions() {
		if action.GetSubresource() == "finalize" {
			t.Error("Reconcile(content finalizer) releases Namespace finalizer, want retained")
		}
	}
}

func TestReconcileReturnsDeletionCause(t *testing.T) {
	r, kubeClient, metadataClient := reconcileFixture(t, terminatingNamespace())
	wantErr := errors.New("discovery unavailable")
	discoverResources := func(context.Context) ([]*metav1.APIResourceList, error) {
		return nil, wantErr
	}
	r.deleter = deletion.New(kubeClient.CoreV1().Namespaces(), metadataClient, discoverResources)

	result, err := r.Reconcile(t.Context(), namespaceRequest())
	if !errors.Is(err, wantErr) || result.RequeueAfter != 0 {
		t.Errorf("Reconcile(discovery failure) = %v, %v, want cause preserved for framework retry", result, err)
	}
}

func TestReconcileReturnsReadCause(t *testing.T) {
	r, kubeClient, metadataClient := reconcileFixture(t, terminatingNamespace())
	wantErr := errors.New("cache read unavailable")
	r.reader = fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Get: func(
			context.Context,
			client.WithWatch,
			client.ObjectKey,
			client.Object,
			...client.GetOption,
		) error {
			return wantErr
		},
	}).Build()

	result, err := r.Reconcile(t.Context(), namespaceRequest())
	if !errors.Is(err, wantErr) || result.RequeueAfter != 0 {
		t.Errorf("Reconcile(read failure) = %v, %v, want read cause preserved", result, err)
	}

	if len(kubeClient.Actions()) != 0 || len(metadataClient.Actions()) != 0 {
		t.Error("Reconcile(read failure) accesses deletion clients, want stopped before cleanup")
	}
}

func TestNamespaceEvents(t *testing.T) {
	active := terminatingNamespace()
	active.DeletionTimestamp = nil
	deleting := terminatingNamespace()
	statusOnly := deleting.DeepCopy()
	statusOnly.Status.Conditions = []corev1.NamespaceCondition{{
		Type:   corev1.NamespaceContentRemaining,
		Status: corev1.ConditionTrue,
	}}
	specFinalizer := deleting.DeepCopy()
	specFinalizer.Spec.Finalizers = []corev1.FinalizerName{"example.com/retain"}
	metadataFinalizer := deleting.DeepCopy()
	metadataFinalizer.Finalizers = nil
	replacement := deleting.DeepCopy()
	replacement.UID = "replacement"
	labelOnly := deleting.DeepCopy()
	labelOnly.Labels = map[string]string{"display": "changed"}
	tests := []struct {
		name     string
		previous *corev1.Namespace
		current  *corev1.Namespace
		want     bool
	}{
		{
			name:     "deletion starts",
			previous: active,
			current:  deleting,
			want:     true,
		},
		{
			name:     "status only",
			previous: deleting,
			current:  statusOnly,
		},
		{
			name:     "spec finalizer",
			previous: deleting,
			current:  specFinalizer,
			want:     true,
		},
		{
			name:     "metadata finalizer",
			previous: deleting,
			current:  metadataFinalizer,
			want:     true,
		},
		{
			name:     "replacement",
			previous: deleting,
			current:  replacement,
			want:     true,
		},
		{
			name:     "label only",
			previous: deleting,
			current:  labelOnly,
		},
		{
			name:     "active update",
			previous: active,
			current:  active,
		},
	}
	filter := namespaceEvents()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter.Update(event.UpdateEvent{ObjectOld: tt.previous, ObjectNew: tt.current}); got != tt.want {
				t.Errorf("namespaceEvents.Update(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}

	if filter.Create(event.CreateEvent{Object: active}) || !filter.Create(event.CreateEvent{Object: deleting}) {
		t.Error("namespaceEvents.Create() does not distinguish Active and existing Terminating namespaces")
	}

	if filter.Delete(event.DeleteEvent{Object: deleting}) || filter.Generic(event.GenericEvent{Object: deleting}) {
		t.Error("namespaceEvents() accepts removed or generic objects, want ignored")
	}
}

func reconcileFixture(
	t *testing.T,
	namespace *corev1.Namespace,
	content ...runtime.Object,
) (*Reconciler, *kubefake.Clientset, *metadatafake.FakeMetadataClient) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	readerBuilder := fake.NewClientBuilder().WithScheme(scheme)
	var namespaces []runtime.Object
	if namespace != nil {
		readerBuilder.WithObjects(namespace)
		namespaces = append(namespaces, namespace)
	}

	kubeClient := kubefake.NewClientset(namespaces...)
	metadataScheme := metadatafake.NewTestScheme()
	if err := metav1.AddMetaToScheme(metadataScheme); err != nil {
		t.Fatalf("AddMetaToScheme() error = %v", err)
	}

	metadataClient := metadatafake.NewSimpleMetadataClient(metadataScheme, content...)
	discoverResources := func(context.Context) ([]*metav1.APIResourceList, error) {
		return nil, nil
	}
	deleter := deletion.New(kubeClient.CoreV1().Namespaces(), metadataClient, discoverResources)

	return New(readerBuilder.Build(), deleter), kubeClient, metadataClient
}

func terminatingNamespace() *corev1.Namespace {
	deletedAt := metav1.NewTime(time.Now().Add(-time.Minute))

	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-a",
			UID:               "original",
			DeletionTimestamp: &deletedAt,
			// controller-runtime fake 不认识 spec.finalizers，要求 metadata finalizer 保留删除对象。
			Finalizers: []string{"example.com/metadata-retain"},
		},
		Spec: corev1.NamespaceSpec{
			Finalizers: []corev1.FinalizerName{corev1.FinalizerKubernetes, "example.com/retain"},
		},
		Status: corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating},
	}
}

func namespaceRequest() ctrl.Request {
	return ctrl.Request{NamespacedName: client.ObjectKey{Name: "team-a"}}
}
