package install_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lgc202/ingate/pkg/apis/gateway"
	"github.com/lgc202/ingate/pkg/apis/gateway/install"
	"github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestInstall(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := install.Install(scheme); err != nil {
		t.Fatalf("install.Install() error = %v, want nil", err)
	}

	for _, version := range []schema.GroupVersion{gateway.SchemeGroupVersion, v1alpha1.SchemeGroupVersion} {
		for _, kind := range []string{"Gateway", "HTTPRoute", "Backend"} {
			gvk := version.WithKind(kind)
			if _, err := scheme.New(gvk); err != nil {
				t.Errorf("Scheme.New(%s) error = %v, want nil", gvk, err)
			}
		}
		for _, kind := range []string{"Route", "Upstream"} {
			if gvk := version.WithKind(kind); scheme.Recognizes(gvk) {
				t.Errorf("Scheme still registers removed resource %s", gvk)
			}
		}
	}

	versions := scheme.PrioritizedVersionsForGroup(gateway.GroupName)
	if len(versions) != 1 || versions[0] != v1alpha1.SchemeGroupVersion {
		t.Errorf("Scheme.PrioritizedVersionsForGroup(%q) = %v, want [%s]",
			gateway.GroupName, versions, v1alpha1.SchemeGroupVersion)
	}
}
