package deletion

import (
	"context"
	"errors"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/fake"
	metadatafake "k8s.io/client-go/metadata/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/retry"
)

func TestNewDoesNotAccessAPI(t *testing.T) {
	kubeClient := fake.NewClientset()
	metadataClient := metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme())
	called := false
	discoverResources := func(context.Context) ([]*metav1.APIResourceList, error) {
		called = true
		return nil, nil
	}
	New(kubeClient.CoreV1().Namespaces(), metadataClient, discoverResources)
	if called || len(kubeClient.Actions()) != 0 || len(metadataClient.Actions()) != 0 {
		t.Error("New() performs discovery or client operations, want construction only")
	}
}

func TestDeleteIgnoresInactiveRequests(t *testing.T) {
	active := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	finalized := deletingNamespace()
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
			var objects []runtime.Object
			if tt.namespace != nil {
				objects = append(objects, tt.namespace)
			}

			kubeClient := fake.NewClientset(objects...)
			metadataClient := metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme())
			called := false
			discoverResources := func(context.Context) ([]*metav1.APIResourceList, error) {
				called = true
				return nil, nil
			}
			deleter := New(kubeClient.CoreV1().Namespaces(), metadataClient, discoverResources)

			if err := deleter.Delete(t.Context(), "team-a"); err != nil {
				t.Errorf("Delete(%s) error = %v, want nil", tt.name, err)
			}

			if called || len(metadataClient.Actions()) != 0 {
				t.Errorf("Delete(%s) calls discovery or metadata client, want skipped cleanup", tt.name)
			}

			assertNoFinalize(t, kubeClient)
		})
	}
}

func TestDeleteEmptyNamespacePreservesOtherFinalizer(t *testing.T) {
	deleter, kubeClient, _ := deletionFixture(t)
	if err := deleter.Delete(t.Context(), "team-a"); err != nil {
		t.Fatalf("Delete(empty team-a) error = %v, want nil", err)
	}

	got, err := kubeClient.CoreV1().Namespaces().Get(t.Context(), "team-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(team-a) error = %v", err)
	}

	want := []corev1.FinalizerName{"example.com/retain"}
	if !slices.Equal(got.Spec.Finalizers, want) {
		t.Errorf("Delete(empty team-a) finalizers = %v, want %v", got.Spec.Finalizers, want)
	}
}

func TestDeleteReadErrorPreservesCause(t *testing.T) {
	deleter, kubeClient, metadataClient := deletionFixture(t)
	wantErr := errors.New("namespace read unavailable")
	kubeClient.PrependReactor("get", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})

	if err := deleter.Delete(t.Context(), "team-a"); !errors.Is(err, wantErr) {
		t.Errorf("Delete(read failure) error = %v, want cause %v", err, wantErr)
	}

	if len(metadataClient.Actions()) != 0 {
		t.Errorf("Delete(read failure) metadata actions = %v, want none", metadataClient.Actions())
	}

	assertNoFinalize(t, kubeClient)
}

func TestDeleteFallbackAndNamespaceIsolation(t *testing.T) {
	deleter, kubeClient, metadataClient := deletionFixture(t, metadataObject("team-a"), metadataObject("team-b"))
	deleter.discoverResourcesFn = func(context.Context) ([]*metav1.APIResourceList, error) {
		return []*metav1.APIResourceList{{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "widgets",
					Namespaced: true,
					Verbs:      metav1.Verbs{"list", "delete"},
				},
				{
					Name:  "clusters",
					Verbs: metav1.Verbs{"list", "delete"},
				},
				{
					Name:       "widgets/status",
					Namespaced: true,
					Verbs:      metav1.Verbs{"delete"},
				},
				{
					Name:       "read-only",
					Namespaced: true,
					Verbs:      metav1.Verbs{"list"},
				},
			},
		}}, nil
	}

	if err := deleter.Delete(t.Context(), "team-a"); err != nil {
		t.Fatalf("Delete(team-a fallback) error = %v, want nil", err)
	}

	deleted := false
	for _, action := range metadataClient.Actions() {
		if action.GetNamespace() != "team-a" || action.GetResource() != widgetResource() || action.GetVerb() == "delete-collection" {
			t.Errorf("Delete(team-a fallback) metadata action = %v, want only team-a widgets List/Delete", action)
		}

		deleted = deleted || action.GetVerb() == "delete"
	}

	if !deleted {
		t.Error("Delete(team-a fallback) has no per-item delete, want fallback executed")
	}

	if _, err := metadataClient.Resource(widgetResource()).Namespace("team-b").Get(t.Context(), "edge", metav1.GetOptions{}); err != nil {
		t.Errorf("Get(team-b/edge) error = %v, want unchanged object", err)
	}

	if !hasFinalize(kubeClient) {
		t.Error("Delete(team-a fallback) does not finalize, want completed cleanup")
	}
}

func TestDeleteCollectionFallsBackOnMethodNotSupported(t *testing.T) {
	deleter, _, metadataClient := deletionFixture(t, metadataObject("team-a"))
	metadataClient.PrependReactor("delete-collection", "widgets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewMethodNotSupported(widgetResource().GroupResource(), "deletecollection")
	})

	if err := deleter.Delete(t.Context(), "team-a"); err != nil {
		t.Fatalf("Delete(team-a unsupported collection) error = %v, want nil", err)
	}

	if _, err := metadataClient.Resource(widgetResource()).Namespace("team-a").Get(t.Context(), "edge", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(team-a/edge) error = %v, want NotFound after fallback", err)
	}
}

func TestDeleteCollectionClearsContent(t *testing.T) {
	deleter, kubeClient, metadataClient := deletionFixture(t, metadataObject("team-a"), metadataObject("team-b"))
	collections := 0
	metadataClient.PrependReactor("delete-collection", "widgets", func(action clienttesting.Action) (bool, runtime.Object, error) {
		collections++
		// Metadata fake 不模拟批量删除；显式移除目标对象，保留另一空间的同名对象。
		err := metadataClient.Tracker().Delete(widgetResource(), action.GetNamespace(), "edge")

		return true, &metav1.Status{Status: metav1.StatusSuccess}, err
	})

	if err := deleter.Delete(t.Context(), "team-a"); err != nil {
		t.Fatalf("Delete(collection) error = %v, want nil", err)
	}

	if collections != 1 || !hasFinalize(kubeClient) {
		t.Errorf("Delete(collection) calls = %d, finalized = %v, want one collection delete and finalize", collections, hasFinalize(kubeClient))
	}

	for _, action := range metadataClient.Actions() {
		if action.GetNamespace() != "team-a" || action.GetVerb() == "delete" {
			t.Errorf("Delete(collection) action = %v, want only team-a List/DeleteCollection", action)
		}
	}

	if _, err := metadataClient.Resource(widgetResource()).Namespace("team-b").Get(t.Context(), "edge", metav1.GetOptions{}); err != nil {
		t.Errorf("Get(team-b/edge) error = %v, want object retained", err)
	}
}

func TestDeleteWaitsForContentFinalizers(t *testing.T) {
	item := metadataObject("team-a")
	item.Finalizers = []string{"example.com/content"}
	deleter, kubeClient, metadataClient := deletionFixture(t, item)
	metadataClient.PrependReactor("delete-collection", "widgets", func(clienttesting.Action) (bool, runtime.Object, error) {
		// 模拟 API 已接收 DELETE，但对象仍由自己的 finalizer 保留。
		return true, &metav1.Status{Status: metav1.StatusSuccess}, nil
	})

	err := deleter.Delete(t.Context(), "team-a")
	remaining, ok := errors.AsType[*ResourcesRemainingError](err)
	if !ok || remaining.Estimate != finalizerEstimateSeconds {
		t.Fatalf("Delete(team-a pending content) error = %v, want ResourcesRemainingError with estimate %d", err, finalizerEstimateSeconds)
	}

	assertNoFinalize(t, kubeClient)
	assertNamespaceCondition(t, kubeClient, corev1.NamespaceContentRemaining, corev1.ConditionTrue)
	assertNamespaceCondition(t, kubeClient, corev1.NamespaceFinalizersRemaining, corev1.ConditionTrue)
}

func TestDeletePartialDiscoveryBlocksFinalize(t *testing.T) {
	deleter, kubeClient, metadataClient := deletionFixture(t)
	wantErr := &discovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{
		{Group: "missing.example.com", Version: "v1"}: errors.New("discovery unavailable"),
	}}
	deleter.discoverResourcesFn = func(context.Context) ([]*metav1.APIResourceList, error) {
		return widgetDiscovery(), wantErr
	}

	if err := deleter.Delete(t.Context(), "team-a"); !errors.Is(err, wantErr) {
		t.Errorf("Delete(partial discovery) error = %v, want cause %v", err, wantErr)
	}

	if len(metadataClient.Actions()) == 0 {
		t.Error("Delete(partial discovery) performs no known-resource cleanup, want progress on discovered resources")
	}

	assertNoFinalize(t, kubeClient)
	assertNamespaceCondition(t, kubeClient, corev1.NamespaceDeletionDiscoveryFailure, corev1.ConditionTrue)
}

func TestDeleteContentErrorBlocksFinalize(t *testing.T) {
	deleter, kubeClient, metadataClient := deletionFixture(t)
	wantErr := errors.New("metadata service unavailable")
	metadataClient.PrependReactor("delete-collection", "widgets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})

	if err := deleter.Delete(t.Context(), "team-a"); !errors.Is(err, wantErr) {
		t.Errorf("Delete(content failure) error = %v, want cause %v", err, wantErr)
	}

	assertNoFinalize(t, kubeClient)
	assertNamespaceCondition(t, kubeClient, corev1.NamespaceDeletionContentFailure, corev1.ConditionTrue)
}

func TestDeleteInvalidGroupVersionBlocksFinalize(t *testing.T) {
	deleter, kubeClient, _ := deletionFixture(t)
	deleter.discoverResourcesFn = func(context.Context) ([]*metav1.APIResourceList, error) {
		resources := widgetDiscovery()
		resources[0].GroupVersion = "invalid/group/version"

		return resources, nil
	}

	if err := deleter.Delete(t.Context(), "team-a"); err == nil {
		t.Error("Delete(invalid GroupVersion) error = nil, want parsing failure")
	}

	assertNoFinalize(t, kubeClient)
	assertNamespaceCondition(t, kubeClient, corev1.NamespaceDeletionGVParsingFailure, corev1.ConditionTrue)
}

func TestDeleteListErrorBlocksFinalize(t *testing.T) {
	deleter, kubeClient, metadataClient := deletionFixture(t)
	wantErr := errors.New("metadata list unavailable")
	metadataClient.PrependReactor("list", "widgets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})

	if err := deleter.Delete(t.Context(), "team-a"); !errors.Is(err, wantErr) {
		t.Errorf("Delete(list failure) error = %v, want cause %v", err, wantErr)
	}

	assertNoFinalize(t, kubeClient)
	assertNamespaceCondition(t, kubeClient, corev1.NamespaceDeletionContentFailure, corev1.ConditionTrue)
}

func TestDeleteUnexpectedResidualBlocksFinalize(t *testing.T) {
	deleter, kubeClient, metadataClient := deletionFixture(t, metadataObject("team-a"))
	metadataClient.PrependReactor("delete-collection", "widgets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, &metav1.Status{Status: metav1.StatusSuccess}, nil
	})

	err := deleter.Delete(t.Context(), "team-a")
	_, waiting := errors.AsType[*ResourcesRemainingError](err)
	if err == nil || waiting {
		t.Errorf("Delete(unexplained residual) error = %v, want operation failure rather than finalizer wait", err)
	}

	assertNoFinalize(t, kubeClient)
	assertNamespaceCondition(t, kubeClient, corev1.NamespaceContentRemaining, corev1.ConditionTrue)
	assertNamespaceCondition(t, kubeClient, corev1.NamespaceDeletionContentFailure, corev1.ConditionTrue)
}

func TestDeleteStatusErrorBlocksFinalize(t *testing.T) {
	deleter, kubeClient, _ := deletionFixture(t)
	wantErr := errors.New("status write unavailable")
	kubeClient.PrependReactor("update", "namespaces", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "status" {
			return true, nil, wantErr
		}

		return false, nil, nil
	})

	if err := deleter.Delete(t.Context(), "team-a"); !errors.Is(err, wantErr) {
		t.Errorf("Delete(status failure) error = %v, want cause %v", err, wantErr)
	}

	assertNoFinalize(t, kubeClient)
}

func TestDeleteFinalizeConflictUIDChange(t *testing.T) {
	deleter, kubeClient, _ := deletionFixture(t)
	finalizeCalls := 0
	kubeClient.PrependReactor("create", "namespaces", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "finalize" {
			return false, nil, nil
		}

		finalizeCalls++
		replacement := deletingNamespace()
		replacement.UID = "replacement"
		if err := kubeClient.Tracker().Update(corev1.SchemeGroupVersion.WithResource("namespaces"), replacement, ""); err != nil {
			return true, nil, err
		}

		return true, nil, apierrors.NewConflict(corev1.Resource("namespaces"), "team-a", errors.New("concurrent replacement"))
	})

	if err := deleter.Delete(t.Context(), "team-a"); err == nil {
		t.Error("Delete(replaced Namespace) error = nil, want UID guard rejection")
	}

	if finalizeCalls != 1 {
		t.Errorf("Delete(replaced Namespace) finalize calls = %d, want 1 without writing replacement", finalizeCalls)
	}

	got, err := kubeClient.CoreV1().Namespaces().Get(t.Context(), "team-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(replacement) error = %v", err)
	}

	if got.UID != "replacement" || !slices.Contains(got.Spec.Finalizers, corev1.FinalizerKubernetes) {
		t.Errorf("Delete(replaced Namespace) result = %v/%v, want replacement with kubernetes finalizer", got.UID, got.Spec.Finalizers)
	}
}

func TestDeleteFinalizeErrorPreservesCause(t *testing.T) {
	deleter, kubeClient, _ := deletionFixture(t)
	wantErr := errors.New("finalize unavailable")
	kubeClient.PrependReactor("create", "namespaces", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "finalize" {
			return true, nil, wantErr
		}

		return false, nil, nil
	})

	if err := deleter.Delete(t.Context(), "team-a"); !errors.Is(err, wantErr) {
		t.Errorf("Delete(finalize failure) error = %v, want cause %v", err, wantErr)
	}

	namespace, err := kubeClient.CoreV1().Namespaces().Get(t.Context(), "team-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(team-a) error = %v, want nil", err)
	}

	if !slices.Contains(namespace.Spec.Finalizers, corev1.FinalizerKubernetes) {
		t.Errorf("Delete(finalize failure) finalizers = %v, want kubernetes finalizer retained", namespace.Spec.Finalizers)
	}
}

func TestDeleteConflictRetriesAreBounded(t *testing.T) {
	deleter, kubeClient, _ := deletionFixture(t)
	finalizeCalls := 0
	kubeClient.PrependReactor("create", "namespaces", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "finalize" {
			return false, nil, nil
		}

		finalizeCalls++

		return true, nil, apierrors.NewConflict(corev1.Resource("namespaces"), "team-a", errors.New("concurrent write"))
	})

	if err := deleter.Delete(t.Context(), "team-a"); !apierrors.IsConflict(err) {
		t.Errorf("Delete(permanent conflict) error = %v, want Conflict", err)
	}

	if finalizeCalls != retry.DefaultRetry.Steps {
		t.Errorf("Delete(permanent conflict) calls = %d, want bounded %d", finalizeCalls, retry.DefaultRetry.Steps)
	}
}

func TestDeleteConflictRecovers(t *testing.T) {
	deleter, kubeClient, _ := deletionFixture(t)
	finalizeCalls := 0
	kubeClient.PrependReactor("create", "namespaces", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "finalize" {
			return false, nil, nil
		}

		finalizeCalls++
		if finalizeCalls == 1 {
			return true, nil, apierrors.NewConflict(corev1.Resource("namespaces"), "team-a", errors.New("concurrent update"))
		}

		return false, nil, nil
	})

	if err := deleter.Delete(t.Context(), "team-a"); err != nil {
		t.Fatalf("Delete(recoverable conflict) error = %v, want nil", err)
	}

	if finalizeCalls != 2 {
		t.Errorf("Delete(recoverable conflict) calls = %d, want two attempts", finalizeCalls)
	}

	got, err := kubeClient.CoreV1().Namespaces().Get(t.Context(), "team-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(team-a) error = %v", err)
	}

	if !slices.Equal(got.Spec.Finalizers, []corev1.FinalizerName{"example.com/retain"}) {
		t.Errorf("Delete(recoverable conflict) finalizers = %v, want only custom finalizer", got.Spec.Finalizers)
	}
}

func TestDeleteCanceled(t *testing.T) {
	deleter, kubeClient, _ := deletionFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := deleter.Delete(ctx, "team-a"); !errors.Is(err, context.Canceled) {
		t.Errorf("Delete(canceled) error = %v, want context.Canceled", err)
	}

	assertNoFinalize(t, kubeClient)
}

func TestDeletePropagatesDiscoveryContext(t *testing.T) {
	deleter, kubeClient, _ := deletionFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	called := false
	deleter.discoverResourcesFn = func(got context.Context) ([]*metav1.APIResourceList, error) {
		called = true
		if got != ctx {
			t.Errorf("Delete(team-a) discovery Context = %v, want caller Context %v", got, ctx)
		}

		cancel()

		return nil, got.Err()
	}

	if err := deleter.Delete(ctx, "team-a"); !errors.Is(err, context.Canceled) {
		t.Errorf("Delete(canceled discovery) error = %v, want context.Canceled", err)
	}

	if !called {
		t.Error("Delete(team-a) skips discovery, want discovery to observe cancellation")
	}

	assertNoFinalize(t, kubeClient)
}

func deletionFixture(
	t *testing.T,
	objects ...runtime.Object,
) (*Deleter, *fake.Clientset, *metadatafake.FakeMetadataClient) {
	t.Helper()

	kubeClient := fake.NewClientset(deletingNamespace())
	scheme := metadatafake.NewTestScheme()
	if err := metav1.AddMetaToScheme(scheme); err != nil {
		t.Fatalf("AddMetaToScheme() error = %v", err)
	}

	metadataClient := metadatafake.NewSimpleMetadataClient(scheme, objects...)
	discoverResources := func(context.Context) ([]*metav1.APIResourceList, error) {
		return widgetDiscovery(), nil
	}
	deleter := New(kubeClient.CoreV1().Namespaces(), metadataClient, discoverResources)

	return deleter, kubeClient, metadataClient
}

func deletingNamespace() *corev1.Namespace {
	deletedAt := metav1.Now()

	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team-a",
			UID:               "original",
			ResourceVersion:   "1",
			DeletionTimestamp: &deletedAt,
		},
		Spec: corev1.NamespaceSpec{
			Finalizers: []corev1.FinalizerName{corev1.FinalizerKubernetes, "example.com/retain"},
		},
		Status: corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating},
	}
}

func metadataObject(namespace string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Widget"},
		ObjectMeta: metav1.ObjectMeta{Name: "edge", Namespace: namespace},
	}
}

func widgetResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}
}

func widgetDiscovery() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{{
		GroupVersion: "example.com/v1",
		APIResources: []metav1.APIResource{{
			Name:       "widgets",
			Namespaced: true,
			Verbs:      metav1.Verbs{"list", "delete", "deletecollection"},
		}},
	}}
}

func assertNoFinalize(t *testing.T, kubeClient *fake.Clientset) {
	t.Helper()

	if hasFinalize(kubeClient) {
		t.Error("Delete() calls Namespace finalize, want finalizer retained")
	}
}

func hasFinalize(kubeClient *fake.Clientset) bool {
	for _, action := range kubeClient.Actions() {
		// client-go v0.37.0 的 fake Finalize 记录 create/finalize；真实客户端发送 PUT。
		if action.GetVerb() == "create" && action.GetSubresource() == "finalize" {
			return true
		}
	}

	return false
}

func assertNamespaceCondition(
	t *testing.T,
	kubeClient *fake.Clientset,
	kind corev1.NamespaceConditionType,
	want corev1.ConditionStatus,
) {
	t.Helper()

	got, err := kubeClient.CoreV1().Namespaces().Get(t.Context(), "team-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(team-a) error = %v", err)
	}

	condition := getCondition(got.Status.Conditions, kind)
	if condition == nil || condition.Status != want {
		t.Errorf("Delete(team-a) condition %s = %v, want status %s", kind, condition, want)
	}
}
