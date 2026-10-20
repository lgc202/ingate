package controllermanager

import (
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

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
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Namespace"), meta.RESTScopeRoot)

	config := &Config{
		RESTConfig:     &rest.Config{Host: "https://127.0.0.1:6443"},
		XDSBindAddress: "127.0.0.1:0",
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
