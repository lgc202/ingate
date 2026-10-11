package apiserver

import (
	"testing"

	openapispec "k8s.io/kube-openapi/pkg/validation/spec"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	generatedopenapi "github.com/lgc202/ingate/pkg/generated/openapi"
)

func TestNewOptions(t *testing.T) {
	options, err := NewOptions()
	if err != nil {
		t.Fatalf("NewOptions() error = %v, want nil", err)
	}

	if got := options.RecommendedOptions.Etcd.StorageConfig.Prefix; got != defaultEtcdPathPrefix {
		t.Errorf("NewOptions() etcd prefix = %q, want %q", got, defaultEtcdPathPrefix)
	}
	gvk := gatewayv1alpha1.SchemeGroupVersion.WithKind("Gateway")
	if _, err := options.scheme.New(gvk); err != nil {
		t.Errorf("NewOptions().scheme.New(%s) error = %v, want nil", gvk, err)
	}
}

func TestOpenAPIDefinitions(t *testing.T) {
	definitions := generatedopenapi.GetOpenAPIDefinitions(
		func(path string) openapispec.Ref { return openapispec.MustCreateRef(path) },
	)

	for _, modelName := range []string{
		gatewayv1alpha1.Gateway{}.OpenAPIModelName(),
		gatewayv1alpha1.HTTPRoute{}.OpenAPIModelName(),
		gatewayv1alpha1.Backend{}.OpenAPIModelName(),
	} {
		if _, ok := definitions[modelName]; !ok {
			t.Errorf("GetOpenAPIDefinitions()[%q] exists = false, want true", modelName)
		}
	}
}
