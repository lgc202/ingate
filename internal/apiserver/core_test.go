package apiserver

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/registry/rest"
	configmapstorage "k8s.io/kubernetes/pkg/registry/core/configmap/storage"
	namespacestorage "k8s.io/kubernetes/pkg/registry/core/namespace/storage"
)

func TestCoreAPIGroupInfo(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme() error = %v", err)
	}

	storage := map[string]rest.Storage{
		"namespaces":          &namespacestorage.REST{},
		"namespaces/status":   &namespacestorage.StatusREST{},
		"namespaces/finalize": &namespacestorage.FinalizeREST{},
		"configmaps":          &configmapstorage.REST{},
	}
	info := coreAPIGroupInfo(scheme, storage)
	if info.Scheme != scheme {
		t.Errorf("coreAPIGroupInfo() scheme = %p, want %p", info.Scheme, scheme)
	}
	if len(info.PrioritizedVersions) != 1 || info.PrioritizedVersions[0] != corev1.SchemeGroupVersion {
		t.Errorf("coreAPIGroupInfo() versions = %v, want [%s]", info.PrioritizedVersions, corev1.SchemeGroupVersion)
	}
	versionedStorage := info.VersionedResourcesStorageMap[corev1.SchemeGroupVersion.Version]
	if len(info.VersionedResourcesStorageMap) != 1 || len(versionedStorage) != len(storage) {
		t.Errorf("coreAPIGroupInfo() storage = %v, want only supported core/v1 storage", info.VersionedResourcesStorageMap)
	}
	for name, want := range storage {
		if got := versionedStorage[name]; got != want {
			t.Errorf("coreAPIGroupInfo() storage[%q] = %v, want %v", name, got, want)
		}
	}
	for _, resource := range []string{"pods", "secrets", "services"} {
		if _, installed := versionedStorage[resource]; installed {
			t.Errorf("coreAPIGroupInfo() installs %q, want only Namespace/ConfigMap storage", resource)
		}
	}

	values, err := info.ParameterCodec.EncodeParameters(
		&metav1.ListOptions{FieldSelector: "status.phase=Active"},
		corev1.SchemeGroupVersion,
	)
	if err != nil {
		t.Fatalf("coreAPIGroupInfo().ParameterCodec.EncodeParameters() error = %v", err)
	}
	if got := values.Get("fieldSelector"); got != "status.phase=Active" {
		t.Errorf("coreAPIGroupInfo() fieldSelector = %q, want status.phase=Active", got)
	}
}
