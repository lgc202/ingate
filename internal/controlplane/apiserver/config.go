package apiserver

import (
	"fmt"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"

	authconfig "github.com/lgc202/ingate/internal/controlplane/apiserver/auth"
	controlplaneoptions "github.com/lgc202/ingate/internal/controlplane/apiserver/options"
	gatewayrest "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/rest"
	policyrest "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/policy/rest"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
	ingatescheme "github.com/lgc202/ingate/pkg/apis/scheme"
	ingatestorage "github.com/lgc202/ingate/pkg/apiserver/storage"
	generatedopenapi "github.com/lgc202/ingate/pkg/generated/openapi"
)

// Config carries the mutable generic apiserver configuration.
type Config struct {
	GenericConfig           *genericapiserver.RecommendedConfig
	APIResourceConfigSource serverstorage.APIResourceConfigSource
	RESTOptionsGetter       generic.RESTOptionsGetter
	RESTStorageProviders    []ingatestorage.RESTStorageProvider
}

type completedConfig struct {
	GenericConfig           genericapiserver.CompletedConfig
	APIResourceConfigSource serverstorage.APIResourceConfigSource
	RESTOptionsGetter       generic.RESTOptionsGetter
	RESTStorageProviders    []ingatestorage.RESTStorageProvider
}

// CompletedConfig is the immutable, ready-to-build server config.
type CompletedConfig struct {
	*completedConfig
}

func NewConfig(opts controlplaneoptions.CompletedOptions) (*Config, error) {
	resourceConfigSource := newAPIResourceConfigSource()
	genericConfig, err := BuildGenericConfig(opts, resourceConfigSource)
	if err != nil {
		return nil, err
	}

	return &Config{
		GenericConfig:           genericConfig,
		APIResourceConfigSource: resourceConfigSource,
		RESTOptionsGetter:       genericConfig.Config.RESTOptionsGetter,
		RESTStorageProviders: []ingatestorage.RESTStorageProvider{
			gatewayrest.RESTStorageProvider{},
			policyrest.RESTStorageProvider{},
		},
	}, nil
}

func BuildGenericConfig(opts controlplaneoptions.CompletedOptions, resourceConfigSource *serverstorage.ResourceConfig) (*genericapiserver.RecommendedConfig, error) {
	genericConfig := genericapiserver.NewRecommendedConfig(ingatescheme.Codecs)
	genericConfig.Config.EnableDiscovery = true
	genericConfig.Config.MergedResourceConfig = resourceConfigSource

	namer := openapi.NewDefinitionNamer(ingatescheme.Scheme)
	getDefinitions := generatedopenapi.GetOpenAPIDefinitions
	genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(getDefinitions, namer)
	genericConfig.OpenAPIConfig.Info.Title = "Ingate"
	genericConfig.OpenAPIConfig.Info.Version = "v1alpha1"
	genericConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(getDefinitions, namer)
	genericConfig.OpenAPIV3Config.Info.Title = "Ingate"
	genericConfig.OpenAPIV3Config.Info.Version = "v1alpha1"

	if err := opts.GenericServerRunOptions.ApplyTo(&genericConfig.Config); err != nil {
		return nil, err
	}

	if err := opts.SecureServing.ApplyTo(&genericConfig.Config.SecureServing, &genericConfig.Config.LoopbackClientConfig); err != nil {
		return nil, err
	}

	if err := opts.Etcd.ApplyTo(&genericConfig.Config); err != nil {
		return nil, err
	}

	if err := applyAdmission(genericConfig, opts); err != nil {
		return nil, err
	}

	authenticator, err := authconfig.NewRequestAuthenticator(
		opts.Auth.AdminToken,
		opts.Auth.AdminUser,
		opts.Auth.AdminGroups,
		opts.Auth.ViewerToken,
		opts.Auth.ViewerUser,
		opts.Auth.ViewerGroups,
		opts.Auth.AnonymousPaths,
	)
	if err != nil {
		return nil, err
	}
	genericConfig.Config.Authentication.Authenticator = authenticator

	authorizer, err := authconfig.NewAuthorizer(opts.Auth.AnonymousPaths, opts.Auth.AuthorizationPolicyFile)
	if err != nil {
		return nil, err
	}
	genericConfig.Config.Authorization.Authorizer = authorizer

	return genericConfig, nil
}

func applyAdmission(genericConfig *genericapiserver.RecommendedConfig, opts controlplaneoptions.CompletedOptions) error {
	pluginNames := controlplaneoptions.EnabledAdmissionPluginNames(opts.Admission)
	pluginsConfigProvider, err := admission.ReadAdmissionConfiguration(pluginNames, opts.Admission.ConfigFile, nil)
	if err != nil {
		return fmt.Errorf("failed to read admission configuration: %w", err)
	}

	admissionChain, err := opts.Admission.Plugins.NewFromPlugins(
		pluginNames,
		pluginsConfigProvider,
		admission.PluginInitializers{},
		opts.Admission.Decorators,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize admission chain: %w", err)
	}

	genericConfig.Config.AdmissionControl = admissionChain
	return nil
}

func (c *Config) Complete() CompletedConfig {
	return CompletedConfig{completedConfig: &completedConfig{
		GenericConfig:           c.GenericConfig.Complete(),
		APIResourceConfigSource: c.APIResourceConfigSource,
		RESTOptionsGetter:       c.RESTOptionsGetter,
		RESTStorageProviders:    c.RESTStorageProviders,
	}}
}

func newAPIResourceConfigSource() *serverstorage.ResourceConfig {
	resourceConfig := serverstorage.NewResourceConfig()
	resourceConfig.EnableVersions(
		gatewayv1alpha1.SchemeGroupVersion,
		policyv1alpha1.SchemeGroupVersion,
	)
	return resourceConfig
}
