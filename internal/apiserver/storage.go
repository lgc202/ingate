package apiserver

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	serverstorage "k8s.io/apiserver/pkg/server/storage"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func newStorageFactory(
	options *genericoptions.EtcdOptions,
	scheme *runtime.Scheme,
	codecs serializer.CodecFactory,
	config *genericapiserver.RecommendedConfig,
) (*serverstorage.DefaultStorageFactory, error) {
	storageConfig := options.StorageConfig
	if storageConfig.StorageObjectCountTracker == nil {
		storageConfig.StorageObjectCountTracker = config.StorageObjectCountTracker
	}

	encoding := serverstorage.NewDefaultResourceEncodingConfigForEffectiveVersion(scheme, config.EffectiveVersion)
	resources := serverstorage.NewResourceConfig()
	resources.EnableVersions(gatewayv1alpha1.SchemeGroupVersion)
	resources.EnableResources(
		corev1.SchemeGroupVersion.WithResource("namespaces"),
		corev1.SchemeGroupVersion.WithResource("configmaps"),
	)

	factory := serverstorage.NewDefaultStorageFactory(
		storageConfig,
		options.DefaultStorageMediaType,
		codecs,
		encoding,
		resources,
		nil,
	)

	overrides, err := genericoptions.ParseEtcdServersOverrides(options.EtcdServersOverrides)
	if err != nil {
		return nil, fmt.Errorf("parse resource storage servers: %w", err)
	}
	for _, override := range overrides {
		factory.SetEtcdLocation(override.GroupResource, override.Servers)
	}

	return factory, nil
}
