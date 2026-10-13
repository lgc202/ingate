//go:build integration

package apiserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/etcd3/testserver"
	"k8s.io/utils/ptr"

	namespacestorage "github.com/lgc202/ingate/internal/apiserver/registry/core/namespace"
	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

// TestNamespaceStorageWrites 验证三类存储入口的字段保护和实际 etcd 筛选。
func TestNamespaceStorageWrites(t *testing.T) {
	ctx := genericapirequest.WithNamespace(t.Context(), "")
	mainREST, statusREST, finalizeREST := newNamespaceStorage(t)
	namespace := createNamespace(t, mainREST, &coreapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{"owner": "platform"}},
		Spec:       corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{"example.com/cleanup"}},
	})
	if namespace.UID == "" || namespace.ResourceVersion == "" || namespace.CreationTimestamp.IsZero() {
		t.Errorf("Create() metadata lacks server identity/version/time: %+v", namespace.ObjectMeta)
	}
	if namespace.Status.Phase != corev1.NamespaceActive || namespace.Labels[corev1.LabelMetadataName] != namespace.Name {
		t.Errorf("Create() phase/labels = %q/%v, want Active and name label", namespace.Status.Phase, namespace.Labels)
	}
	wantFinalizers := []corev1.FinalizerName{"example.com/cleanup", corev1.FinalizerKubernetes}
	if diff := cmp.Diff(wantFinalizers, namespace.Spec.Finalizers); diff != "" {
		t.Errorf("Create() spec.finalizers mismatch (-want +got):\n%s", diff)
	}

	ordinary := namespace.DeepCopy()
	ordinary.Labels["owner"] = "other"
	ordinary.Spec.Finalizers = nil
	ordinary.Status.Phase = corev1.NamespaceTerminating
	namespace = updateNamespace(t, mainREST, ordinary)
	if namespace.Status.Phase != corev1.NamespaceActive || !cmp.Equal(namespace.Spec.Finalizers, wantFinalizers) {
		t.Errorf("ordinary Update() changed protected phase/finalizers: %q/%v", namespace.Status.Phase, namespace.Spec.Finalizers)
	}

	invalidStatus := namespace.DeepCopy()
	invalidStatus.Status.Phase = corev1.NamespaceTerminating
	info := rest.DefaultUpdatedObjectInfo(invalidStatus)
	_, _, err := statusREST.Update(
		ctx,
		namespace.Name,
		info,
		rest.ValidateAllObjectFunc,
		rest.ValidateAllObjectUpdateFunc,
		false,
		&metav1.UpdateOptions{},
	)
	if !apierrors.IsInvalid(err) {
		t.Errorf("status Update(inconsistent phase) error = %v, want Invalid", err)
	}
	status := namespace.DeepCopy()
	status.Spec.Finalizers = nil
	wantConditions := []corev1.NamespaceCondition{{
		Type:               corev1.NamespaceDeletionDiscoveryFailure,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)),
		Reason:             "Discovered",
		Message:            "all resources discovered",
	}}
	status.Status.Conditions = wantConditions
	namespace = updateNamespace(t, statusREST, status)
	if diff := cmp.Diff(wantFinalizers, namespace.Spec.Finalizers); diff != "" {
		t.Errorf("status Update() changed spec.finalizers (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantConditions, namespace.Status.Conditions); diff != "" {
		t.Errorf("status Update() conditions mismatch (-want +got):\n%s", diff)
	}

	finalized := namespace.DeepCopy()
	finalized.Spec.Finalizers = nil
	finalized.Status.Conditions = nil
	namespace = updateNamespace(t, finalizeREST, finalized)
	if len(namespace.Spec.Finalizers) != 0 {
		t.Errorf("finalize Update() spec.finalizers = %v, want empty", namespace.Spec.Finalizers)
	}
	if diff := cmp.Diff(wantConditions, namespace.Status.Conditions); diff != "" {
		t.Errorf("finalize Update() changed conditions (-want +got):\n%s", diff)
	}
	if _, err := mainREST.Get(ctx, namespace.Name, &metav1.GetOptions{}); err != nil {
		t.Errorf("Get(non-deleting finalized Namespace) error = %v, want present", err)
	}

	tests := []struct {
		name          string
		labelSelector labels.Selector
		fieldSelector fields.Selector
		wantCount     int
	}{
		{
			name:          "matching",
			labelSelector: labels.SelectorFromSet(labels.Set{"owner": "other"}),
			fieldSelector: fields.SelectorFromSet(fields.Set{"metadata.name": "team-a", "status.phase": "Active"}),
			wantCount:     1,
		},
		{
			name:          "different_label",
			labelSelector: labels.SelectorFromSet(labels.Set{"owner": "platform"}),
			fieldSelector: fields.Everything(),
		},
		{
			name:          "different_name",
			labelSelector: labels.Everything(),
			fieldSelector: fields.OneTermEqualSelector("metadata.name", "team-b"),
		},
		{
			name:          "different_phase",
			labelSelector: labels.Everything(),
			fieldSelector: fields.OneTermEqualSelector("status.phase", "Terminating"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &metainternalversion.ListOptions{LabelSelector: tt.labelSelector, FieldSelector: tt.fieldSelector}
			obj, err := mainREST.List(ctx, options)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if got := len(obj.(*coreapi.NamespaceList).Items); got != tt.wantCount {
				t.Errorf("List() count = %d, want %d", got, tt.wantCount)
			}
		})
	}

	missing := &coreapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "missing"}}
	for _, storage := range []rest.Updater{statusREST, finalizeREST} {
		info := rest.DefaultUpdatedObjectInfo(missing)
		_, _, err := storage.Update(
			ctx,
			missing.Name,
			info,
			rest.ValidateAllObjectFunc,
			rest.ValidateAllObjectUpdateFunc,
			true,
			&metav1.UpdateOptions{},
		)
		if !apierrors.IsNotFound(err) {
			t.Errorf("%T.Update(missing, forceCreate) error = %v, want NotFound", storage, err)
		}
	}
}

// TestNamespaceStorageDelete 验证冲突、dry-run 和两类 finalizer 对删除的限制。
func TestNamespaceStorageDelete(t *testing.T) {
	ctx := genericapirequest.WithNamespace(t.Context(), "")
	mainREST, _, finalizeREST := newNamespaceStorage(t)
	namespace := createNamespace(t, mainREST, &coreapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a", Finalizers: []string{"example.com/metadata-cleanup"}},
		Spec:       corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{"example.com/cleanup"}},
	})
	for _, preconditions := range []*metav1.Preconditions{
		{UID: ptr.To(types.UID("wrong"))},
		{ResourceVersion: ptr.To("wrong")},
	} {
		options := &metav1.DeleteOptions{Preconditions: preconditions}
		if _, _, err := mainREST.Delete(ctx, namespace.Name, rest.ValidateAllObjectFunc, options); !apierrors.IsConflict(err) {
			t.Errorf("Delete(%+v) error = %v, want Conflict", preconditions, err)
		}
	}
	stored := getNamespace(t, mainREST, namespace.Name)
	if diff := cmp.Diff(namespace, stored); diff != "" {
		t.Errorf("conflicting Delete changed storage (-want +got):\n%s", diff)
	}

	options := &metav1.DeleteOptions{DryRun: []string{metav1.DryRunAll}}
	before := options.DeepCopy()
	obj, deleted, err := mainREST.Delete(ctx, namespace.Name, rest.ValidateAllObjectFunc, options)
	if err != nil {
		t.Fatalf("Delete(dry-run) error = %v", err)
	}
	dryRun := obj.(*coreapi.Namespace)
	if deleted || dryRun.DeletionTimestamp.IsZero() || dryRun.Status.Phase != corev1.NamespaceTerminating {
		t.Errorf("Delete(dry-run) deleted/phase/timestamp = %t/%q/%v, want false/Terminating/nonzero",
			deleted, dryRun.Status.Phase, dryRun.DeletionTimestamp)
	}
	if diff := cmp.Diff(before, options); diff != "" {
		t.Errorf("Delete mutated options (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(namespace, getNamespace(t, mainREST, namespace.Name)); diff != "" {
		t.Errorf("dry-run persisted changes (-want +got):\n%s", diff)
	}

	options = &metav1.DeleteOptions{Preconditions: &metav1.Preconditions{ResourceVersion: &namespace.ResourceVersion}}
	before = options.DeepCopy()
	obj, deleted, err = mainREST.Delete(ctx, namespace.Name, rest.ValidateAllObjectFunc, options)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	namespace = obj.(*coreapi.Namespace)
	if deleted || namespace.DeletionTimestamp.IsZero() || namespace.Status.Phase != corev1.NamespaceTerminating {
		t.Errorf("Delete() deleted/phase/timestamp = %t/%q/%v, want false/Terminating/nonzero",
			deleted, namespace.Status.Phase, namespace.DeletionTimestamp)
	}
	if diff := cmp.Diff(before, options); diff != "" {
		t.Errorf("Delete mutated preconditions (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(namespace, getNamespace(t, mainREST, namespace.Name)); diff != "" {
		t.Errorf("termination state not persisted (-want +got):\n%s", diff)
	}
	if _, deleted, err := mainREST.Delete(ctx, namespace.Name, rest.ValidateAllObjectFunc, nil); err != nil || deleted {
		t.Fatalf("Delete(repeated) = deleted %t, error %v, want false/nil", deleted, err)
	}
	if diff := cmp.Diff(namespace, getNamespace(t, mainREST, namespace.Name)); diff != "" {
		t.Errorf("repeated Delete changed termination state (-want +got):\n%s", diff)
	}

	partial := namespace.DeepCopy()
	partial.Spec.Finalizers = []corev1.FinalizerName{"example.com/cleanup"}
	namespace = updateNamespace(t, finalizeREST, partial)
	if _, err := mainREST.Get(ctx, namespace.Name, &metav1.GetOptions{}); err != nil {
		t.Fatalf("Get(custom spec finalizer remaining) error = %v, want present", err)
	}

	finalized := namespace.DeepCopy()
	finalized.Spec.Finalizers = nil
	namespace = updateNamespace(t, finalizeREST, finalized)
	if _, err := mainREST.Get(ctx, namespace.Name, &metav1.GetOptions{}); err != nil {
		t.Fatalf("Get(metadata finalizer remaining) error = %v, want present", err)
	}
	if _, deleted, err := mainREST.Delete(ctx, namespace.Name, rest.ValidateAllObjectFunc, nil); err != nil || deleted {
		t.Fatalf("Delete(metadata finalizer remaining) = deleted %t, error %v, want false/nil", deleted, err)
	}
	namespace = getNamespace(t, mainREST, namespace.Name)
	namespace.Finalizers = nil
	updateNamespace(t, mainREST, namespace)
	if _, err := mainREST.Get(ctx, namespace.Name, &metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(fully finalized Namespace) error = %v, want NotFound", err)
	}
}

// TestNamespaceStorageFinalize 验证最后一个 spec finalizer 的更新可以完成物理删除。
func TestNamespaceStorageFinalize(t *testing.T) {
	ctx := genericapirequest.WithNamespace(t.Context(), "")
	mainREST, _, finalizeREST := newNamespaceStorage(t)
	namespace := createNamespace(t, mainREST, &coreapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}})
	obj, _, err := mainREST.Delete(ctx, namespace.Name, rest.ValidateAllObjectFunc, nil)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	namespace = obj.(*coreapi.Namespace)
	namespace.Spec.Finalizers = nil
	updateNamespace(t, finalizeREST, namespace)
	if _, err := mainREST.Get(ctx, namespace.Name, &metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(after finalize) error = %v, want NotFound", err)
	}
	recreated := createNamespace(t, mainREST, &coreapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace.Name}})
	if recreated.UID == namespace.UID {
		t.Error("recreated Namespace has old UID, want new identity")
	}
	options := &metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &namespace.UID}}
	if _, _, err := mainREST.Delete(ctx, recreated.Name, rest.ValidateAllObjectFunc, options); !apierrors.IsConflict(err) {
		t.Errorf("Delete(stale UID) error = %v, want Conflict", err)
	}
	if got := getNamespace(t, mainREST, recreated.Name); !got.DeletionTimestamp.IsZero() {
		t.Errorf("Delete(stale UID) marked replacement deleting: %v", got.DeletionTimestamp)
	}
}

// TestNamespaceStorageDeleteValidation 验证删除校验失败不会写入 Terminating。
func TestNamespaceStorageDeleteValidation(t *testing.T) {
	ctx := genericapirequest.WithNamespace(t.Context(), "")
	mainREST, _, _ := newNamespaceStorage(t)
	namespace := createNamespace(t, mainREST, &coreapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}})
	resource := coreapi.SchemeGroupVersion.WithResource("namespaces").GroupResource()
	validationErr := apierrors.NewForbidden(resource, namespace.Name, errors.New("denied"))
	validation := func(_ context.Context, _ runtime.Object) error { return validationErr }
	if _, _, err := mainREST.Delete(ctx, namespace.Name, validation, nil); !apierrors.IsForbidden(err) {
		t.Errorf("Delete(denied) error = %v, want Forbidden", err)
	}
	if diff := cmp.Diff(namespace, getNamespace(t, mainREST, namespace.Name)); diff != "" {
		t.Errorf("denied Delete changed storage (-want +got):\n%s", diff)
	}
}

func newNamespaceStorage(
	t *testing.T,
) (
	*namespacestorage.REST,
	*namespacestorage.StatusREST,
	*namespacestorage.FinalizeREST,
) {
	t.Helper()

	etcd := testserver.RunEtcd(t, nil)
	t.Cleanup(func() {
		if err := etcd.Close(); err != nil {
			t.Errorf("close namespace etcd client: %v", err)
		}
	})

	options, config := storageTestOptions(t)
	etcdOptions := options.RecommendedOptions.Etcd
	etcdOptions.StorageConfig.Transport.ServerList = etcd.Endpoints()
	etcdOptions.EnableWatchCache = false
	factory, err := newStorageFactory(etcdOptions, options.scheme, options.codecs, config)
	if err != nil {
		t.Fatalf("newStorageFactory() error = %v", err)
	}

	getter := etcdOptions.CreateRESTOptionsGetter(factory, nil)
	mainREST, statusREST, finalizeREST, err := namespacestorage.NewREST(options.scheme, getter)
	if err != nil {
		t.Fatalf("NewREST(Namespace) error = %v", err)
	}
	t.Cleanup(mainREST.Destroy)
	return mainREST, statusREST, finalizeREST
}

func createNamespace(
	t *testing.T,
	storage *namespacestorage.REST,
	namespace *coreapi.Namespace,
) *coreapi.Namespace {
	t.Helper()
	ctx := genericapirequest.WithNamespace(t.Context(), "")

	obj, err := storage.Create(ctx, namespace, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(Namespace) error = %v", err)
	}
	return obj.(*coreapi.Namespace)
}

func getNamespace(t *testing.T, storage *namespacestorage.REST, name string) *coreapi.Namespace {
	t.Helper()
	ctx := genericapirequest.WithNamespace(t.Context(), "")

	obj, err := storage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(Namespace %q) error = %v", name, err)
	}
	return obj.(*coreapi.Namespace)
}

func updateNamespace(
	t *testing.T,
	storage rest.Updater,
	namespace *coreapi.Namespace,
) *coreapi.Namespace {
	t.Helper()
	ctx := genericapirequest.WithNamespace(t.Context(), "")

	info := rest.DefaultUpdatedObjectInfo(namespace)
	obj, created, err := storage.Update(
		ctx,
		namespace.Name,
		info,
		rest.ValidateAllObjectFunc,
		rest.ValidateAllObjectUpdateFunc,
		false,
		&metav1.UpdateOptions{},
	)
	if err != nil {
		t.Fatalf("%T.Update(Namespace) error = %v", storage, err)
	}
	if created {
		t.Errorf("%T.Update(Namespace) created object, want false", storage)
	}
	return obj.(*coreapi.Namespace)
}
