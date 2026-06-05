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
	// ResourceRuntimeGroup 表示 RuntimeGroup 单数资源名
	ResourceRuntimeGroup ResourceName = "runtimegroup"
	// ResourceRuntimeGroups 表示 RuntimeGroup 复数资源名
	ResourceRuntimeGroups ResourceName = "runtimegroups"
	// ResourceRuntimeGroupsStatus 表示 RuntimeGroup status 子资源名
	ResourceRuntimeGroupsStatus ResourceName = "runtimegroups/status"
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
	// ResourceRuntimeSnapshot 表示 RuntimeSnapshot 单数资源名
	ResourceRuntimeSnapshot ResourceName = "runtimesnapshot"
	// ResourceRuntimeSnapshots 表示 RuntimeSnapshot 复数资源名
	ResourceRuntimeSnapshots ResourceName = "runtimesnapshots"
	// ResourceAIRoute 表示 AIRoute 单数资源名
	ResourceAIRoute ResourceName = "airoute"
	// ResourceAIRoutes 表示 AIRoute 复数资源名
	ResourceAIRoutes ResourceName = "airoutes"
	// ResourceAIRoutesStatus 表示 AIRoute status 子资源名
	ResourceAIRoutesStatus ResourceName = "airoutes/status"
	// ResourceAIProvider 表示 AIProvider 单数资源名
	ResourceAIProvider ResourceName = "aiprovider"
	// ResourceAIProviders 表示 AIProvider 复数资源名
	ResourceAIProviders ResourceName = "aiproviders"
	// ResourceAIProvidersStatus 表示 AIProvider status 子资源名
	ResourceAIProvidersStatus ResourceName = "aiproviders/status"
	// ResourceAIModel 表示 AIModel 单数资源名
	ResourceAIModel ResourceName = "aimodel"
	// ResourceAIModels 表示 AIModel 复数资源名
	ResourceAIModels ResourceName = "aimodels"
	// ResourceAIModelsStatus 表示 AIModel status 子资源名
	ResourceAIModelsStatus ResourceName = "aimodels/status"
	// ResourceAIPolicy 表示 AIPolicy 单数资源名
	ResourceAIPolicy ResourceName = "aipolicy"
	// ResourceAIPolicies 表示 AIPolicy 复数资源名
	ResourceAIPolicies ResourceName = "aipolicies"
	// ResourceAIPoliciesStatus 表示 AIPolicy status 子资源名
	ResourceAIPoliciesStatus ResourceName = "aipolicies/status"
	// ResourcePlugin 表示 Plugin 单数资源名
	ResourcePlugin ResourceName = "plugin"
	// ResourcePlugins 表示 Plugin 复数资源名
	ResourcePlugins ResourceName = "plugins"
	// ResourcePluginsStatus 表示 Plugin status 子资源名
	ResourcePluginsStatus ResourceName = "plugins/status"
	// ResourcePluginBinding 表示 PluginBinding 单数资源名
	ResourcePluginBinding ResourceName = "pluginbinding"
	// ResourcePluginBindings 表示 PluginBinding 复数资源名
	ResourcePluginBindings ResourceName = "pluginbindings"
	// ResourcePluginBindingsStatus 表示 PluginBinding status 子资源名
	ResourcePluginBindingsStatus ResourceName = "pluginbindings/status"
	// ResourceAuthPolicy 表示 AuthPolicy 单数资源名
	ResourceAuthPolicy ResourceName = "authpolicy"
	// ResourceAuthPolicies 表示 AuthPolicy 复数资源名
	ResourceAuthPolicies ResourceName = "authpolicies"
	// ResourceAuthPoliciesStatus 表示 AuthPolicy status 子资源名
	ResourceAuthPoliciesStatus ResourceName = "authpolicies/status"
	// ResourceRateLimitPolicy 表示 RateLimitPolicy 单数资源名
	ResourceRateLimitPolicy ResourceName = "ratelimitpolicy"
	// ResourceRateLimitPolicies 表示 RateLimitPolicy 复数资源名
	ResourceRateLimitPolicies ResourceName = "ratelimitpolicies"
	// ResourceRateLimitPoliciesStatus 表示 RateLimitPolicy status 子资源名
	ResourceRateLimitPoliciesStatus ResourceName = "ratelimitpolicies/status"
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
		&RuntimeGroup{},
		&RuntimeGroupList{},
		&Route{},
		&RouteList{},
		&AIRoute{},
		&AIRouteList{},
		&Upstream{},
		&UpstreamList{},
		&AIProvider{},
		&AIProviderList{},
		&AIModel{},
		&AIModelList{},
		&AIPolicy{},
		&AIPolicyList{},
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
		&RuntimeSnapshot{},
		&RuntimeSnapshotList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
