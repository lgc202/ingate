package controllermanager

import (
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gatewaycontroller "github.com/lgc202/ingate/internal/controllermanager/controller/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestConfigNew(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Core AddToScheme() error = %v, want nil", err)
	}
	if err := gatewayv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Gateway AddToScheme() error = %v, want nil", err)
	}

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)

	config := &Config{
		RESTConfig:        &rest.Config{Host: "https://127.0.0.1:6443"},
		XDSBindAddress:    "127.0.0.1:0",
		AIConfigNamespace: "control-plane",
		ManagerOptions: ctrl.Options{
			MapperProvider:         func(*rest.Config, *http.Client) (meta.RESTMapper, error) { return mapper, nil },
			Scheme:                 scheme,
			Metrics:                metricsserver.Options{BindAddress: "0"},
			HealthProbeBindAddress: "0",
		},
	}

	manager, err := config.New(nil)
	if err != nil {
		t.Fatalf("Config.New() error = %v, want nil", err)
	}

	if got := manager.GetScheme(); got != scheme {
		t.Errorf("Config.New() Scheme = %p, want %p", got, scheme)
	}
}

func TestConfigNewRejectsInvalidAIConfigNamespace(t *testing.T) {
	for _, namespace := range []string{"", "Control-Plane", "team/a"} {
		t.Run(namespace, func(t *testing.T) {
			config := Config{AIConfigNamespace: namespace}

			manager, err := config.New(nil)

			if err == nil || manager != nil {
				t.Errorf("Config.New(namespace %q) manager/error = %v/%v, want nil manager and validation error", namespace, manager, err)
			}
		})
	}
}

func TestManagerOptionsLimitConfigMapCache(t *testing.T) {
	oldConfigMap := &corev1.ConfigMap{}
	gateway := &gatewayv1alpha1.Gateway{}
	oldScope := cache.ByObject{Namespaces: map[string]cache.Config{"legacy-space": {}}}
	config := Config{
		AIConfigNamespace: "control-plane",
		ManagerOptions: ctrl.Options{Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{oldConfigMap: oldScope, gateway: {}},
		}},
	}

	got := config.managerOptions()

	var configMaps int
	for object, scope := range got.Cache.ByObject {
		if _, isConfigMap := object.(*corev1.ConfigMap); !isConfigMap {
			continue
		}

		configMaps++
		controlPlane, found := scope.Namespaces["control-plane"]
		if len(scope.Namespaces) != 1 || !found {
			t.Fatalf("managerOptions() ConfigMap namespace scope = %+v, want only control-plane", scope.Namespaces)
		}
		selector := controlPlane.FieldSelector
		if selector == nil || !selector.Matches(fields.Set{"metadata.name": gatewaycontroller.AIConfigMapName}) ||
			selector.Matches(fields.Set{"metadata.name": "other"}) {
			t.Errorf("managerOptions() ConfigMap field selector = %v, want only %q", selector, gatewaycontroller.AIConfigMapName)
		}
	}
	if configMaps != 1 {
		t.Errorf("managerOptions() ConfigMap cache registrations = %d, want 1", configMaps)
	}
	if scope, found := got.Cache.ByObject[gateway]; !found || scope.Namespaces != nil || got.Cache.DefaultNamespaces != nil {
		t.Errorf("managerOptions() changed domain-resource scope: %+v", got.Cache)
	}
	if scope, found := config.ManagerOptions.Cache.ByObject[oldConfigMap]; !found || len(scope.Namespaces) != 1 ||
		scope.Namespaces["legacy-space"].FieldSelector != nil {
		t.Errorf("managerOptions() mutated the caller's cache registrations: %+v", config.ManagerOptions.Cache.ByObject)
	}
	if _, found := config.ManagerOptions.Cache.ByObject[oldConfigMap].Namespaces["legacy-space"]; !found {
		t.Error("managerOptions() replaced the caller's ConfigMap namespace scope")
	}
}
