package server

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	aimodelstorage "github.com/lgc202/ingate/internal/apiserver/registry/aimodel"
	aipolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/aipolicy"
	aiproviderstorage "github.com/lgc202/ingate/internal/apiserver/registry/aiprovider"
	airoutestorage "github.com/lgc202/ingate/internal/apiserver/registry/airoute"
	authpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/authpolicy"
	gatewaystorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway"
	pluginstorage "github.com/lgc202/ingate/internal/apiserver/registry/plugin"
	pluginbindingstorage "github.com/lgc202/ingate/internal/apiserver/registry/pluginbinding"
	policybindingstorage "github.com/lgc202/ingate/internal/apiserver/registry/policybinding"
	ratelimitpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/ratelimitpolicy"
	routestorage "github.com/lgc202/ingate/internal/apiserver/registry/route"
	runtimesnapshotstorage "github.com/lgc202/ingate/internal/apiserver/registry/runtimesnapshot"
	upstreamstorage "github.com/lgc202/ingate/internal/apiserver/registry/upstream"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const serverName = "ingate-apiserver"

// ExtraConfig 表示 ingate-apiserver 自己的扩展配置
type ExtraConfig struct {
	Storage map[string]rest.Storage
}

// Config 表示 ingate-apiserver 完整配置
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// CompletedConfig 表示补全后的 ingate-apiserver 配置
type CompletedConfig struct {
	*completedConfig
}

// Server 表示 ingate-apiserver 运行实例
type Server struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

// Complete 补全 ingate-apiserver 配置
func (c *Config) Complete() CompletedConfig {
	return CompletedConfig{&completedConfig{
		GenericConfig: c.GenericConfig.Complete(),
		ExtraConfig:   &c.ExtraConfig,
	}}
}

// New 创建 ingate-apiserver 实例
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*Server, error) {
	genericServer, err := c.GenericConfig.New(serverName, delegationTarget)
	if err != nil {
		return nil, err
	}

	server := &Server{GenericAPIServer: genericServer}
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		gatewayv1.GroupName,
		Scheme,
		runtime.NewParameterCodec(Scheme),
		Codecs,
	)

	storage := c.ExtraConfig.Storage
	if storage == nil {
		storage = map[string]rest.Storage{}
	}
	_, hasGateway := storage[string(gatewayv1.ResourceGateways)]
	_, hasGatewayStatus := storage[string(gatewayv1.ResourceGatewaysStatus)]
	if !hasGateway || !hasGatewayStatus {
		gatewayREST, gatewayStatusREST, err := gatewaystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasGateway {
			storage[string(gatewayv1.ResourceGateways)] = gatewayREST
		}
		if !hasGatewayStatus {
			storage[string(gatewayv1.ResourceGatewaysStatus)] = gatewayStatusREST
		}
	}
	_, hasRoute := storage[string(gatewayv1.ResourceRoutes)]
	_, hasRouteStatus := storage[string(gatewayv1.ResourceRoutesStatus)]
	if !hasRoute || !hasRouteStatus {
		routeREST, routeStatusREST, err := routestorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasRoute {
			storage[string(gatewayv1.ResourceRoutes)] = routeREST
		}
		if !hasRouteStatus {
			storage[string(gatewayv1.ResourceRoutesStatus)] = routeStatusREST
		}
	}
	_, hasUpstream := storage[string(gatewayv1.ResourceUpstreams)]
	_, hasUpstreamStatus := storage[string(gatewayv1.ResourceUpstreamsStatus)]
	if !hasUpstream || !hasUpstreamStatus {
		upstreamREST, upstreamStatusREST, err := upstreamstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasUpstream {
			storage[string(gatewayv1.ResourceUpstreams)] = upstreamREST
		}
		if !hasUpstreamStatus {
			storage[string(gatewayv1.ResourceUpstreamsStatus)] = upstreamStatusREST
		}
	}
	_, hasRuntimeSnapshot := storage[string(gatewayv1.ResourceRuntimeSnapshots)]
	if !hasRuntimeSnapshot {
		runtimeSnapshotREST, err := runtimesnapshotstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		storage[string(gatewayv1.ResourceRuntimeSnapshots)] = runtimeSnapshotREST
	}
	_, hasAIProvider := storage[string(gatewayv1.ResourceAIProviders)]
	_, hasAIProviderStatus := storage[string(gatewayv1.ResourceAIProvidersStatus)]
	if !hasAIProvider || !hasAIProviderStatus {
		aiProviderREST, aiProviderStatusREST, err := aiproviderstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasAIProvider {
			storage[string(gatewayv1.ResourceAIProviders)] = aiProviderREST
		}
		if !hasAIProviderStatus {
			storage[string(gatewayv1.ResourceAIProvidersStatus)] = aiProviderStatusREST
		}
	}
	_, hasAIModel := storage[string(gatewayv1.ResourceAIModels)]
	_, hasAIModelStatus := storage[string(gatewayv1.ResourceAIModelsStatus)]
	if !hasAIModel || !hasAIModelStatus {
		aiModelREST, aiModelStatusREST, err := aimodelstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasAIModel {
			storage[string(gatewayv1.ResourceAIModels)] = aiModelREST
		}
		if !hasAIModelStatus {
			storage[string(gatewayv1.ResourceAIModelsStatus)] = aiModelStatusREST
		}
	}
	_, hasAIRoute := storage[string(gatewayv1.ResourceAIRoutes)]
	_, hasAIRouteStatus := storage[string(gatewayv1.ResourceAIRoutesStatus)]
	if !hasAIRoute || !hasAIRouteStatus {
		aiRouteREST, aiRouteStatusREST, err := airoutestorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasAIRoute {
			storage[string(gatewayv1.ResourceAIRoutes)] = aiRouteREST
		}
		if !hasAIRouteStatus {
			storage[string(gatewayv1.ResourceAIRoutesStatus)] = aiRouteStatusREST
		}
	}
	_, hasAIPolicy := storage[string(gatewayv1.ResourceAIPolicies)]
	_, hasAIPolicyStatus := storage[string(gatewayv1.ResourceAIPoliciesStatus)]
	if !hasAIPolicy || !hasAIPolicyStatus {
		aiPolicyREST, aiPolicyStatusREST, err := aipolicystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasAIPolicy {
			storage[string(gatewayv1.ResourceAIPolicies)] = aiPolicyREST
		}
		if !hasAIPolicyStatus {
			storage[string(gatewayv1.ResourceAIPoliciesStatus)] = aiPolicyStatusREST
		}
	}
	_, hasPlugin := storage[string(gatewayv1.ResourcePlugins)]
	_, hasPluginStatus := storage[string(gatewayv1.ResourcePluginsStatus)]
	if !hasPlugin || !hasPluginStatus {
		pluginREST, pluginStatusREST, err := pluginstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasPlugin {
			storage[string(gatewayv1.ResourcePlugins)] = pluginREST
		}
		if !hasPluginStatus {
			storage[string(gatewayv1.ResourcePluginsStatus)] = pluginStatusREST
		}
	}
	_, hasPluginBinding := storage[string(gatewayv1.ResourcePluginBindings)]
	_, hasPluginBindingStatus := storage[string(gatewayv1.ResourcePluginBindingsStatus)]
	if !hasPluginBinding || !hasPluginBindingStatus {
		pluginBindingREST, pluginBindingStatusREST, err := pluginbindingstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasPluginBinding {
			storage[string(gatewayv1.ResourcePluginBindings)] = pluginBindingREST
		}
		if !hasPluginBindingStatus {
			storage[string(gatewayv1.ResourcePluginBindingsStatus)] = pluginBindingStatusREST
		}
	}
	_, hasAuthPolicy := storage[string(gatewayv1.ResourceAuthPolicies)]
	_, hasAuthPolicyStatus := storage[string(gatewayv1.ResourceAuthPoliciesStatus)]
	if !hasAuthPolicy || !hasAuthPolicyStatus {
		authPolicyREST, authPolicyStatusREST, err := authpolicystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasAuthPolicy {
			storage[string(gatewayv1.ResourceAuthPolicies)] = authPolicyREST
		}
		if !hasAuthPolicyStatus {
			storage[string(gatewayv1.ResourceAuthPoliciesStatus)] = authPolicyStatusREST
		}
	}
	_, hasRateLimitPolicy := storage[string(gatewayv1.ResourceRateLimitPolicies)]
	_, hasRateLimitPolicyStatus := storage[string(gatewayv1.ResourceRateLimitPoliciesStatus)]
	if !hasRateLimitPolicy || !hasRateLimitPolicyStatus {
		rateLimitPolicyREST, rateLimitPolicyStatusREST, err := ratelimitpolicystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasRateLimitPolicy {
			storage[string(gatewayv1.ResourceRateLimitPolicies)] = rateLimitPolicyREST
		}
		if !hasRateLimitPolicyStatus {
			storage[string(gatewayv1.ResourceRateLimitPoliciesStatus)] = rateLimitPolicyStatusREST
		}
	}
	_, hasPolicyBinding := storage[string(gatewayv1.ResourcePolicyBindings)]
	_, hasPolicyBindingStatus := storage[string(gatewayv1.ResourcePolicyBindingsStatus)]
	if !hasPolicyBinding || !hasPolicyBindingStatus {
		policyBindingREST, policyBindingStatusREST, err := policybindingstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		if !hasPolicyBinding {
			storage[string(gatewayv1.ResourcePolicyBindings)] = policyBindingREST
		}
		if !hasPolicyBindingStatus {
			storage[string(gatewayv1.ResourcePolicyBindingsStatus)] = policyBindingStatusREST
		}
	}
	apiGroupInfo.VersionedResourcesStorageMap[gatewayv1.SchemeGroupVersion.Version] = storage

	if err := server.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}
