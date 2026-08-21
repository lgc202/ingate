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
	// AnnotationUpdatedAt 保存资源期望配置最后一次变化的时间
	AnnotationUpdatedAt = GroupName + "/updated-at"
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
	// ResourceCertificate 表示 Certificate 单数资源名
	ResourceCertificate ResourceName = "certificate"
	// ResourceCertificates 表示 Certificate 复数资源名
	ResourceCertificates ResourceName = "certificates"
	// ResourceCertificatesStatus 表示 Certificate status 子资源名
	ResourceCertificatesStatus ResourceName = "certificates/status"
	// ResourceRateLimitPolicy 表示 RateLimitPolicy 单数资源名
	ResourceRateLimitPolicy ResourceName = "ratelimitpolicy"
	// ResourceRateLimitPolicies 表示 RateLimitPolicy 复数资源名
	ResourceRateLimitPolicies ResourceName = "ratelimitpolicies"
	// ResourceRateLimitPoliciesStatus 表示 RateLimitPolicy status 子资源名
	ResourceRateLimitPoliciesStatus ResourceName = "ratelimitpolicies/status"
	// ResourceIPRestrictionPolicy 表示 IPRestrictionPolicy 单数资源名
	ResourceIPRestrictionPolicy ResourceName = "iprestrictionpolicy"
	// ResourceIPRestrictionPolicies 表示 IPRestrictionPolicy 复数资源名
	ResourceIPRestrictionPolicies ResourceName = "iprestrictionpolicies"
	// ResourceIPRestrictionPoliciesStatus 表示 IPRestrictionPolicy status 子资源名
	ResourceIPRestrictionPoliciesStatus ResourceName = "iprestrictionpolicies/status"
	// ResourceCaller 表示 Caller 单数资源名
	ResourceCaller ResourceName = "caller"
	// ResourceCallers 表示 Caller 复数资源名
	ResourceCallers ResourceName = "callers"
	// ResourceCallersStatus 表示 Caller status 子资源名
	ResourceCallersStatus ResourceName = "callers/status"
)

var (
	// SchemeGroupVersion 表示 Ingate API 组版本
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}
	// SchemeBuilder 注册 Ingate API 类型
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// localSchemeBuilder 供生成的转换代码注册版本转换函数
	localSchemeBuilder = &SchemeBuilder
	// AddToScheme 将 Ingate API 类型注册到 Scheme
	AddToScheme = localSchemeBuilder.AddToScheme
)

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
		&Certificate{},
		&CertificateList{},
		&RateLimitPolicy{},
		&RateLimitPolicyList{},
		&IPRestrictionPolicy{},
		&IPRestrictionPolicyList{},
		&Caller{},
		&CallerList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
