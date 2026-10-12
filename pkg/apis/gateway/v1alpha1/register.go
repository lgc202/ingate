package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

const (
	// GroupName 是代理网关资源的 API Group。
	GroupName = gateway.GroupName
	// Version 是代理网关资源的第一个外部版本。
	Version = "v1alpha1"
)

var (
	// SchemeGroupVersion 标识代理网关资源的外部版本。
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}
	localSchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes, addDefaultingFuncs)
	// AddToScheme 将代理网关外部类型及转换函数注册到 Scheme。
	AddToScheme = localSchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		SchemeGroupVersion,
		&Gateway{}, &GatewayList{},
		&HTTPRoute{}, &HTTPRouteList{},
		&Backend{}, &BackendList{},
		&AIBackend{}, &AIBackendList{},
		&ModelRoute{}, &ModelRouteList{},
		&MCPRoute{}, &MCPRouteList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
