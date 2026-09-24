package gateway

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName 是代理网关资源使用的 API Group。
const GroupName = "gateway.ingate.io"

var (
	// SchemeGroupVersion 标识代理网关资源的内部版本。
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}
	schemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme 将代理网关内部类型注册到 Scheme。
	AddToScheme = schemeBuilder.AddToScheme
)

// Resource 返回当前 API Group 中资源的标识。
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		SchemeGroupVersion,
		&Gateway{}, &GatewayList{},
		&Route{}, &RouteList{},
		&Upstream{}, &UpstreamList{},
	)
	return nil
}
