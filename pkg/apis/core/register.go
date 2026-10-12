package core

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// SchemeGroupVersion 标识 Kubernetes 核心组的内部版本。
	SchemeGroupVersion = schema.GroupVersion{Version: runtime.APIVersionInternal}
	localSchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme 注册 Ingate 提供的核心组内部类型。
	AddToScheme = localSchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion, &Namespace{}, &NamespaceList{})
	return nil
}
