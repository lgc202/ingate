package v1

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

// ResourceName 表示 gateway.ingate.io 下的资源名
type ResourceName string

const (
	// ResourceGateway 表示 Gateway 单数资源名
	ResourceGateway ResourceName = "gateway"
	// ResourceGateways 表示 Gateway 复数资源名
	ResourceGateways ResourceName = "gateways"
	// ResourceGatewaysStatus 表示 Gateway status 子资源名
	ResourceGatewaysStatus ResourceName = "gateways/status"
	// ResourceRoute 表示 Route 单数资源名
	ResourceRoute ResourceName = "route"
	// ResourceRoutes 表示 Route 复数资源名
	ResourceRoutes ResourceName = "routes"
	// ResourceRoutesStatus 表示 Route status 子资源名
	ResourceRoutesStatus ResourceName = "routes/status"
	// ResourceUpstream 表示 Upstream 单数资源名
	ResourceUpstream ResourceName = "upstream"
	// ResourceUpstreams 表示 Upstream 复数资源名
	ResourceUpstreams ResourceName = "upstreams"
	// ResourceUpstreamsStatus 表示 Upstream status 子资源名
	ResourceUpstreamsStatus ResourceName = "upstreams/status"
	// ResourceRateLimitPolicy 表示 RateLimitPolicy 单数资源名
	ResourceRateLimitPolicy ResourceName = "ratelimitpolicy"
	// ResourceRateLimitPolicies 表示 RateLimitPolicy 复数资源名
	ResourceRateLimitPolicies ResourceName = "ratelimitpolicies"
	// ResourceRateLimitPoliciesStatus 表示 RateLimitPolicy status 子资源名
	ResourceRateLimitPoliciesStatus ResourceName = "ratelimitpolicies/status"
	// ResourceAccessControlPolicy 表示 AccessControlPolicy 单数资源名
	ResourceAccessControlPolicy ResourceName = "accesscontrolpolicy"
	// ResourceAccessControlPolicies 表示 AccessControlPolicy 复数资源名
	ResourceAccessControlPolicies ResourceName = "accesscontrolpolicies"
	// ResourceAccessControlPoliciesStatus 表示 AccessControlPolicy status 子资源名
	ResourceAccessControlPoliciesStatus ResourceName = "accesscontrolpolicies/status"
	// ResourcePolicyBinding 表示 PolicyBinding 单数资源名
	ResourcePolicyBinding ResourceName = "policybinding"
	// ResourcePolicyBindings 表示 PolicyBinding 复数资源名
	ResourcePolicyBindings ResourceName = "policybindings"
	// ResourcePolicyBindingsStatus 表示 PolicyBinding status 子资源名
	ResourcePolicyBindingsStatus ResourceName = "policybindings/status"
)

// SchemeGroupVersion 表示 Ingate API 组版本
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

// SchemeBuilder 注册 Ingate API 类型
var SchemeBuilder runtime.SchemeBuilder

var localSchemeBuilder = &SchemeBuilder

// AddToScheme 将 Ingate API 类型注册到 Scheme
var AddToScheme = localSchemeBuilder.AddToScheme

func init() {
	localSchemeBuilder.Register(addKnownTypes, addConversionFuncs)
}

// Resource 返回 gateway.ingate.io 资源名
func Resource(name ResourceName) schema.GroupResource {
	return SchemeGroupVersion.WithResource(string(name)).GroupResource()
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Gateway{},
		&GatewayList{},
		&Route{},
		&RouteList{},
		&Upstream{},
		&UpstreamList{},
		&RateLimitPolicy{},
		&RateLimitPolicyList{},
		&AccessControlPolicy{},
		&AccessControlPolicyList{},
		&PolicyBinding{},
		&PolicyBindingList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
