package apiserver

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/compatibility"
	flowcontrolrequest "k8s.io/apiserver/pkg/util/flowcontrol/request"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestStorageFactoryCodecs(t *testing.T) {
	options, config := storageTestOptions(t)
	factory, err := newStorageFactory(options.RecommendedOptions.Etcd, options.scheme, options.codecs, config)
	if err != nil {
		t.Fatalf("newStorageFactory() error = %v", err)
	}

	namespace := &coreapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{corev1.LabelMetadataName: "team-a"}},
		Spec:       coreapi.NamespaceSpec{Finalizers: []coreapi.FinalizerName{coreapi.FinalizerKubernetes}},
		Status:     coreapi.NamespaceStatus{Phase: coreapi.NamespaceActive},
	}
	gateway := &gatewayapi.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "edge"}}
	tests := []struct {
		name       string
		resource   schema.GroupResource
		object     runtime.Object
		storageGVK schema.GroupVersionKind
	}{
		{
			name:       "namespace",
			resource:   corev1.SchemeGroupVersion.WithResource("namespaces").GroupResource(),
			object:     namespace,
			storageGVK: corev1.SchemeGroupVersion.WithKind("Namespace"),
		},
		{
			name:       "namespace_list",
			resource:   corev1.SchemeGroupVersion.WithResource("namespaces").GroupResource(),
			object:     &coreapi.NamespaceList{Items: []coreapi.Namespace{*namespace.DeepCopy()}},
			storageGVK: corev1.SchemeGroupVersion.WithKind("NamespaceList"),
		},
		{
			name:     "configmap",
			resource: corev1.SchemeGroupVersion.WithResource("configmaps").GroupResource(),
			object: &coreapi.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "runtime"},
				Data:       map[string]string{"config.json": "{}"},
			},
			storageGVK: corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		},
		{
			name:     "configmap_list",
			resource: corev1.SchemeGroupVersion.WithResource("configmaps").GroupResource(),
			object: &coreapi.ConfigMapList{Items: []coreapi.ConfigMap{{
				ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "runtime"},
				Data:       map[string]string{},
				BinaryData: map[string][]byte{"bytes": {0, 255}},
			}}},
			storageGVK: corev1.SchemeGroupVersion.WithKind("ConfigMapList"),
		},
		{
			name:       "gateway",
			resource:   gatewayapi.Resource("gateways"),
			object:     gateway,
			storageGVK: gatewayv1alpha1.SchemeGroupVersion.WithKind("Gateway"),
		},
		{
			name:       "gateway_list",
			resource:   gatewayapi.Resource("gateways"),
			object:     &gatewayapi.GatewayList{Items: []gatewayapi.Gateway{*gateway.DeepCopy()}},
			storageGVK: gatewayv1alpha1.SchemeGroupVersion.WithKind("GatewayList"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := factory.NewConfig(tt.resource, tt.object)
			if err != nil {
				t.Fatalf("Factory.NewConfig(%s) error = %v", tt.resource, err)
			}

			before := tt.object.DeepCopyObject()
			data, err := runtime.Encode(storage.Codec, tt.object)
			if err != nil {
				t.Fatalf("Encode(%T) error = %v", tt.object, err)
			}

			var typeMeta metav1.TypeMeta
			if err := json.Unmarshal(data, &typeMeta); err != nil {
				t.Fatalf("Unmarshal(%T metadata) error = %v", tt.object, err)
			}
			wantMeta := metav1.TypeMeta{APIVersion: tt.storageGVK.GroupVersion().String(), Kind: tt.storageGVK.Kind}
			if diff := cmp.Diff(wantMeta, typeMeta); diff != "" {
				t.Errorf("storage TypeMeta mismatch (-want +got):\n%s", diff)
			}

			decoded, err := runtime.Decode(storage.Codec, data)
			if err != nil {
				t.Fatalf("Decode(%T) error = %v", tt.object, err)
			}
			if diff := cmp.Diff(before, decoded); diff != "" {
				t.Errorf("decoded internal object mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(before, tt.object); diff != "" {
				t.Errorf("storage encoding mutated input (-want +got):\n%s", diff)
			}
			if got := factory.ResourcePrefix(tt.resource); got != tt.resource.Resource {
				t.Errorf("Factory.ResourcePrefix(%s) = %q, want %q", tt.resource, got, tt.resource.Resource)
			}
		})
	}

	if _, err := factory.NewConfig(schema.GroupResource{Group: "unknown.ingate.io", Resource: "objects"}, namespace); err == nil {
		t.Error("Factory.NewConfig(unknown group) error = nil, want unregistered group")
	}
}

func TestStorageFactoryResourceServers(t *testing.T) {
	options, config := storageTestOptions(t)
	etcd := options.RecommendedOptions.Etcd
	etcd.StorageConfig.Transport.ServerList = []string{"http://127.0.0.1:2379"}
	etcd.EtcdServersOverrides = []string{"gateway.ingate.io/gateways#http://127.0.0.1:2381;http://127.0.0.1:2382"}
	factory, err := newStorageFactory(etcd, options.scheme, options.codecs, config)
	if err != nil {
		t.Fatalf("newStorageFactory() error = %v", err)
	}

	tests := []struct {
		resource schema.GroupResource
		object   runtime.Object
		servers  []string
	}{
		{
			resource: gatewayapi.Resource("gateways"),
			object:   &gatewayapi.Gateway{},
			servers:  []string{"http://127.0.0.1:2381", "http://127.0.0.1:2382"},
		},
		{
			resource: gatewayapi.Resource("backends"),
			object:   &gatewayapi.Backend{},
			servers:  []string{"http://127.0.0.1:2379"},
		},
		{
			resource: corev1.SchemeGroupVersion.WithResource("namespaces").GroupResource(),
			object:   &coreapi.Namespace{},
			servers:  []string{"http://127.0.0.1:2379"},
		},
	}

	for _, tt := range tests {
		storage, err := factory.NewConfig(tt.resource, tt.object)
		if err != nil {
			t.Fatalf("Factory.NewConfig(%s) error = %v", tt.resource, err)
		}
		if diff := cmp.Diff(tt.servers, storage.Transport.ServerList); diff != "" {
			t.Errorf("Factory.NewConfig(%s) servers mismatch (-want +got):\n%s", tt.resource, diff)
		}
	}
	if got := len(factory.Configs()); got != 2 {
		t.Errorf("Factory.Configs() count = %d, want default and overridden backend", got)
	}

	etcd.EtcdServersOverrides = []string{"malformed"}
	_, wantErr := genericoptions.ParseEtcdServersOverrides(etcd.EtcdServersOverrides)
	if wantErr == nil {
		t.Fatal("ParseEtcdServersOverrides(malformed) error = nil, want parse error")
	}
	if _, err := newStorageFactory(etcd, options.scheme, options.codecs, config); !errors.Is(err, wantErr) {
		t.Errorf("newStorageFactory(malformed servers override) error = %v, want wrapped %v", err, wantErr)
	}
}

func TestStorageFactoryConfig(t *testing.T) {
	options, config := storageTestOptions(t)
	etcd := options.RecommendedOptions.Etcd
	etcd.StorageConfig.Prefix = "/isolated"
	config.StorageObjectCountTracker = flowcontrolrequest.NewStorageObjectCountTracker()
	factory, err := newStorageFactory(etcd, options.scheme, options.codecs, config)
	if err != nil {
		t.Fatalf("newStorageFactory() error = %v", err)
	}
	resource := corev1.SchemeGroupVersion.WithResource("namespaces").GroupResource()
	storage, err := factory.NewConfig(resource, &coreapi.Namespace{})
	if err != nil {
		t.Fatalf("Factory.NewConfig(Namespace) error = %v", err)
	}
	if storage.Prefix != "/isolated" {
		t.Errorf("storage prefix = %q, want /isolated", storage.Prefix)
	}
	if storage.StorageObjectCountTracker != config.StorageObjectCountTracker {
		t.Error("storage count tracker differs from GenericConfig, want shared tracker")
	}
	if etcd.StorageConfig.StorageObjectCountTracker != nil {
		t.Error("newStorageFactory() mutated original EtcdOptions count tracker")
	}
	if factory.APIResourceConfigSource.ResourceEnabled(corev1.SchemeGroupVersion.WithResource("pods")) {
		t.Error("Factory enables Pod storage, want only supported core resources")
	}
	for _, resource := range []string{"namespaces", "configmaps"} {
		if !factory.APIResourceConfigSource.ResourceEnabled(corev1.SchemeGroupVersion.WithResource(resource)) {
			t.Errorf("Factory enables %q = false, want true", resource)
		}
	}

	explicitTracker := flowcontrolrequest.NewStorageObjectCountTracker()
	etcd.StorageConfig.StorageObjectCountTracker = explicitTracker
	factory, err = newStorageFactory(etcd, options.scheme, options.codecs, config)
	if err != nil {
		t.Fatalf("newStorageFactory(explicit count tracker) error = %v", err)
	}
	storage, err = factory.NewConfig(resource, &coreapi.Namespace{})
	if err != nil {
		t.Fatalf("Factory.NewConfig(explicit count tracker) error = %v", err)
	}
	if storage.StorageObjectCountTracker != explicitTracker || etcd.StorageConfig.StorageObjectCountTracker != explicitTracker {
		t.Error("storage count tracker replaced explicit EtcdOptions tracker")
	}
}

func storageTestOptions(t *testing.T) (*Options, *genericapiserver.RecommendedConfig) {
	t.Helper()

	options, err := NewOptions()
	if err != nil {
		t.Fatalf("NewOptions() error = %v", err)
	}
	config := genericapiserver.NewRecommendedConfig(options.codecs)
	config.EffectiveVersion = compatibility.DefaultBuildEffectiveVersion()
	return options, config
}
