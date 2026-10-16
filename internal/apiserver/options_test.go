package apiserver

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiopenapi "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/kube-openapi/pkg/builder"
	"k8s.io/kube-openapi/pkg/builder3"
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
		corev1.Namespace{}.OpenAPIModelName(),
		corev1.NamespaceList{}.OpenAPIModelName(),
		corev1.NamespaceSpec{}.OpenAPIModelName(),
		corev1.NamespaceStatus{}.OpenAPIModelName(),
		corev1.NamespaceCondition{}.OpenAPIModelName(),
		gatewayv1alpha1.Gateway{}.OpenAPIModelName(),
		gatewayv1alpha1.HTTPRoute{}.OpenAPIModelName(),
		gatewayv1alpha1.Backend{}.OpenAPIModelName(),
	} {
		if _, ok := definitions[modelName]; !ok {
			t.Errorf("GetOpenAPIDefinitions()[%q] exists = false, want true", modelName)
		}
	}

	for modelName, definition := range definitions {
		for _, dependency := range definition.Dependencies {
			if _, ok := definitions[dependency]; !ok {
				t.Errorf("GetOpenAPIDefinitions()[%q] dependency %q missing, want defined", modelName, dependency)
			}
		}
	}
}

func TestNamespaceOpenAPIV2(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme() error = %v", err)
	}

	definitionNamer := apiopenapi.NewDefinitionNamer(scheme)
	config := genericapiserver.DefaultOpenAPIConfig(
		generatedopenapi.GetOpenAPIDefinitions,
		definitionNamer,
	)
	swagger, err := builder.BuildOpenAPIDefinitionsForResources(
		config,
		corev1.Namespace{}.OpenAPIModelName(),
		corev1.NamespaceList{}.OpenAPIModelName(),
	)
	if err != nil {
		t.Fatalf("BuildOpenAPIDefinitionsForResources(Namespace v2) error = %v", err)
	}

	for _, modelName := range []string{
		corev1.Namespace{}.OpenAPIModelName(),
		corev1.NamespaceList{}.OpenAPIModelName(),
		corev1.NamespaceSpec{}.OpenAPIModelName(),
		corev1.NamespaceStatus{}.OpenAPIModelName(),
		corev1.NamespaceCondition{}.OpenAPIModelName(),
	} {
		if _, ok := swagger.Definitions[modelName]; !ok {
			t.Errorf("Namespace v2 definitions[%q] missing, want defined", modelName)
		}
	}

	if _, ok := swagger.Definitions[corev1.Pod{}.OpenAPIModelName()]; ok {
		t.Error("Namespace v2 definitions include Pod, want only roots and dependencies")
	}
}

func TestNamespaceOpenAPIV3(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme() error = %v", err)
	}

	definitionNamer := apiopenapi.NewDefinitionNamer(scheme)
	config := genericapiserver.DefaultOpenAPIV3Config(
		generatedopenapi.GetOpenAPIDefinitions,
		definitionNamer,
	)
	schemas, err := builder3.BuildOpenAPIDefinitionsForResources(
		config,
		corev1.Namespace{}.OpenAPIModelName(),
		corev1.NamespaceList{}.OpenAPIModelName(),
	)
	if err != nil {
		t.Fatalf("BuildOpenAPIDefinitionsForResources(Namespace v3) error = %v", err)
	}

	for _, modelName := range []string{
		corev1.Namespace{}.OpenAPIModelName(),
		corev1.NamespaceList{}.OpenAPIModelName(),
		corev1.NamespaceSpec{}.OpenAPIModelName(),
		corev1.NamespaceStatus{}.OpenAPIModelName(),
		corev1.NamespaceCondition{}.OpenAPIModelName(),
	} {
		if _, ok := schemas[modelName]; !ok {
			t.Errorf("Namespace v3 schemas[%q] missing, want defined", modelName)
		}
	}

	if _, ok := schemas[corev1.Pod{}.OpenAPIModelName()]; ok {
		t.Error("Namespace v3 schemas include Pod, want only roots and dependencies")
	}
}
