//go:build integration

package apiserver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kube-openapi/pkg/spec3"
	openapispec "k8s.io/kube-openapi/pkg/validation/spec"
)

func TestCoreAPIHTTPContract(t *testing.T) {
	config := startAPIServer(t)
	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		t.Fatalf("NewDiscoveryClientForConfig() error = %v", err)
	}

	resources, err := client.ServerResourcesForGroupVersion("v1")
	if err != nil {
		t.Fatalf("ServerResourcesForGroupVersion(v1) error = %v", err)
	}
	verbs := make(map[string]metav1.Verbs)
	for _, resource := range resources.APIResources {
		verbs[resource.Name] = resource.Verbs
		if resource.Namespaced || resource.Kind != "Namespace" {
			t.Errorf("Discovery(%q) scope/kind = %t/%q, want false/Namespace",
				resource.Name, resource.Namespaced, resource.Kind)
		}
	}
	wantVerbs := map[string]metav1.Verbs{
		"namespaces":          {"create", "delete", "get", "list", "patch", "update", "watch"},
		"namespaces/status":   {"get", "patch", "update"},
		"namespaces/finalize": {"update"},
	}
	if diff := cmp.Diff(wantVerbs, verbs); diff != "" {
		t.Errorf("Discovery(v1) verbs mismatch (-want +got):\n%s", diff)
	}

	data, err := client.RESTClient().Get().AbsPath("/openapi/v2").DoRaw(t.Context())
	if err != nil {
		t.Fatalf("GET /openapi/v2 error = %v", err)
	}
	var swagger openapispec.Swagger
	if err := json.Unmarshal(data, &swagger); err != nil {
		t.Fatalf("decode OpenAPI v2 error = %v", err)
	}
	if swagger.Paths == nil {
		t.Fatal("OpenAPI v2 paths = nil, want installed API paths")
	}
	for _, path := range []string{
		"/api/v1/namespaces",
		"/api/v1/namespaces/{name}",
		"/api/v1/namespaces/{name}/status",
		"/api/v1/namespaces/{name}/finalize",
	} {
		if _, ok := swagger.Paths.Paths[path]; !ok {
			t.Errorf("OpenAPI v2 path %q missing, want installed", path)
		}
	}
	if _, ok := swagger.Definitions[corev1.Pod{}.OpenAPIModelName()]; ok {
		t.Error("OpenAPI v2 includes Pod, want only installed resources and dependencies")
	}
	if _, ok := swagger.Definitions[corev1.Namespace{}.OpenAPIModelName()]; !ok {
		t.Error("OpenAPI v2 lacks Namespace, want installed definition")
	}

	data, err = client.RESTClient().Get().AbsPath("/openapi/v3/api/v1").DoRaw(t.Context())
	if err != nil {
		t.Fatalf("GET /openapi/v3/api/v1 error = %v", err)
	}
	var document spec3.OpenAPI
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode OpenAPI v3 error = %v", err)
	}
	if document.Components == nil {
		t.Fatal("OpenAPI v3 components = nil, want installed schemas")
	}
	if _, ok := document.Components.Schemas[corev1.Namespace{}.OpenAPIModelName()]; !ok {
		t.Error("OpenAPI v3 lacks Namespace, want installed definition")
	}
	if _, ok := document.Components.Schemas[corev1.Pod{}.OpenAPIModelName()]; ok {
		t.Error("OpenAPI v3 includes Pod, want only installed resources and dependencies")
	}

	if err := client.RESTClient().Get().AbsPath("/api/v1/pods").Do(t.Context()).Error(); !apierrors.IsNotFound(err) {
		t.Errorf("GET /api/v1/pods error = %v, want NotFound", err)
	}
	if err := client.RESTClient().Get().AbsPath("/apis/v1/namespaces").Do(t.Context()).Error(); !apierrors.IsNotFound(err) {
		t.Errorf("GET /apis/v1/namespaces error = %v, want NotFound", err)
	}
}

func TestNamespaceHTTPContract(t *testing.T) {
	client, err := typedcorev1.NewForConfig(startAPIServer(t))
	if err != nil {
		t.Fatalf("core/v1.NewForConfig() error = %v", err)
	}
	ctx := t.Context()
	resources := client.Namespaces()
	input := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	namespace, err := resources.Create(ctx, input, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(Namespace) error = %v", err)
	}
	if namespace.UID == "" || namespace.ResourceVersion == "" || namespace.CreationTimestamp.IsZero() {
		t.Errorf("Create(Namespace) metadata = %+v, want server identity/version/time", namespace.ObjectMeta)
	}
	if namespace.Status.Phase != corev1.NamespaceActive || namespace.Labels[corev1.LabelMetadataName] != namespace.Name {
		t.Errorf("Create(Namespace) phase/labels = %q/%v, want Active and name label", namespace.Status.Phase, namespace.Labels)
	}
	wantFinalizers := []corev1.FinalizerName{corev1.FinalizerKubernetes}
	if diff := cmp.Diff(wantFinalizers, namespace.Spec.Finalizers); diff != "" {
		t.Errorf("Create(Namespace) finalizers mismatch (-want +got):\n%s", diff)
	}
	if _, err := resources.Create(ctx, input, metav1.CreateOptions{}); !apierrors.IsAlreadyExists(err) {
		t.Errorf("Create(duplicate Namespace) error = %v, want AlreadyExists", err)
	}

	updated := namespace.DeepCopy()
	updated.Labels["owner"] = "platform"
	updated.Spec.Finalizers = nil
	updated.Status.Phase = corev1.NamespaceTerminating
	updated, err = resources.Update(ctx, updated, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Update(Namespace) error = %v", err)
	}
	if updated.Status.Phase != corev1.NamespaceActive || !cmp.Equal(updated.Spec.Finalizers, wantFinalizers) {
		t.Errorf("Update(Namespace) phase/finalizers = %q/%v, want Active/%v",
			updated.Status.Phase, updated.Spec.Finalizers, wantFinalizers)
	}
	if _, err := resources.Update(ctx, namespace, metav1.UpdateOptions{}); !apierrors.IsConflict(err) {
		t.Errorf("Update(stale Namespace) error = %v, want Conflict", err)
	}

	listOptions := metav1.ListOptions{LabelSelector: "owner=platform", FieldSelector: "status.phase=Active"}
	listed, err := resources.List(ctx, listOptions)
	if err != nil {
		t.Fatalf("List(matching Namespace) error = %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Name != namespace.Name {
		t.Errorf("List(matching Namespace) items = %v, want only team-a", listed.Items)
	}
	listed, err = resources.List(ctx, metav1.ListOptions{FieldSelector: "status.phase=Terminating"})
	if err != nil {
		t.Fatalf("List(nonmatching phase) error = %v", err)
	}
	if len(listed.Items) != 0 {
		t.Errorf("List(nonmatching phase) items = %v, want empty", listed.Items)
	}

	status := updated.DeepCopy()
	status.Spec.Finalizers = nil
	wantConditions := []corev1.NamespaceCondition{{
		Type:               corev1.NamespaceDeletionDiscoveryFailure,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)),
		Reason:             "Discovered",
		Message:            "all resources discovered",
	}}
	status.Status.Conditions = wantConditions
	status, err = resources.UpdateStatus(ctx, status, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateStatus(Namespace) error = %v", err)
	}
	if diff := cmp.Diff(wantFinalizers, status.Spec.Finalizers); diff != "" {
		t.Errorf("UpdateStatus(Namespace) finalizers mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantConditions, status.Status.Conditions); diff != "" {
		t.Errorf("UpdateStatus(Namespace) conditions mismatch (-want +got):\n%s", diff)
	}

	options := metav1.DeleteOptions{DryRun: []string{metav1.DryRunAll}}
	if err := resources.Delete(ctx, namespace.Name, options); err != nil {
		t.Fatalf("Delete(Namespace, dry-run) error = %v", err)
	}
	stored, err := resources.Get(ctx, namespace.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(Namespace after dry-run) error = %v", err)
	}
	if stored.DeletionTimestamp != nil || stored.ResourceVersion != status.ResourceVersion {
		t.Errorf("Delete(dry-run) persisted changes: timestamp/version = %v/%q, want nil/%q",
			stored.DeletionTimestamp, stored.ResourceVersion, status.ResourceVersion)
	}
	if err := resources.Delete(ctx, namespace.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Delete(Namespace) error = %v", err)
	}
	stored, err = resources.Get(ctx, namespace.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(terminating Namespace) error = %v", err)
	}
	if stored.DeletionTimestamp.IsZero() || stored.Status.Phase != corev1.NamespaceTerminating {
		t.Errorf("Delete(Namespace) timestamp/phase = %v/%q, want nonzero/Terminating", stored.DeletionTimestamp, stored.Status.Phase)
	}

	patch := []byte(`{"spec":{"finalizers":[]}}`)
	_, err = resources.Patch(
		ctx,
		namespace.Name,
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
		"finalize",
	)
	if !apierrors.IsMethodNotSupported(err) {
		t.Errorf("Patch(Namespace finalize) error = %v, want MethodNotSupported", err)
	}
	stored.Spec.Finalizers = nil
	if _, err := resources.Finalize(ctx, stored, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Finalize(Namespace) error = %v", err)
	}
	if _, err := resources.Get(ctx, namespace.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(finalized Namespace) error = %v, want NotFound", err)
	}
}
