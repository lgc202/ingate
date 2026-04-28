package resource

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// GroupName 表示 Ingate API 资源组名
	GroupName = "gateway.ingate.io"
	// Version 表示 Ingate API 版本
	Version = "v1"
)

// SchemeGroupVersion 表示 Ingate API 组版本
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

// SchemeBuilder 注册 Ingate API 类型
var SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

// AddToScheme 将 Ingate API 类型注册到 Scheme
var AddToScheme = SchemeBuilder.AddToScheme

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Gateway{},
		&GatewayList{},
		&Route{},
		&RouteList{},
		&AIRoute{},
		&AIRouteList{},
		&Upstream{},
		&UpstreamList{},
		&AIProvider{},
		&AIProviderList{},
		&Plugin{},
		&PluginList{},
		&AuthPolicy{},
		&AuthPolicyList{},
		&RateLimitPolicy{},
		&RateLimitPolicyList{},
		&PolicyBinding{},
		&PolicyBindingList{},
		&PluginBinding{},
		&PluginBindingList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
