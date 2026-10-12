package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme 注册标准 Namespace 和 NamespaceList。
	AddToScheme = localSchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Namespace{}, &corev1.NamespaceList{})
	metav1.AddToGroupVersion(scheme, corev1.SchemeGroupVersion)
	return nil
}
