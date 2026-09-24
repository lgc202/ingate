package apiserver

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	gatewayregistry "github.com/lgc202/ingate/internal/apiserver/registry/gateway"
	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func installGatewayAPIGroup(
	server *genericapiserver.GenericAPIServer,
	scheme *runtime.Scheme,
	optionsGetter generic.RESTOptionsGetter,
) error {
	storage, err := gatewayregistry.NewStorage(scheme, optionsGetter)
	if err != nil {
		return fmt.Errorf("create %s API group storage: %w", gatewayapi.GroupName, err)
	}

	apiGroupInfo := gatewayAPIGroupInfo(scheme, storage)
	if err := server.InstallAPIGroup(&apiGroupInfo); err != nil {
		for _, resourceStorage := range storage {
			resourceStorage.Destroy()
		}

		return fmt.Errorf("install %s API group: %w", gatewayapi.GroupName, err)
	}

	return nil
}

func gatewayAPIGroupInfo(
	scheme *runtime.Scheme,
	storage map[string]rest.Storage,
) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		gatewayapi.GroupName,
		scheme,
		runtime.NewParameterCodec(scheme),
		serializer.NewCodecFactory(scheme),
	)
	apiGroupInfo.VersionedResourcesStorageMap[gatewayv1alpha1.Version] = storage

	return apiGroupInfo
}
