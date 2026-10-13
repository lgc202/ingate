package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes, addDefaultingFuncs, addFieldLabelConversions)
	// AddToScheme 注册已安装的标准核心类型、默认值和字段转换。
	AddToScheme = localSchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(corev1.SchemeGroupVersion,
		&corev1.Namespace{}, &corev1.NamespaceList{},
		&corev1.ConfigMap{}, &corev1.ConfigMapList{},
	)
	metav1.AddToGroupVersion(scheme, corev1.SchemeGroupVersion)
	return nil
}
