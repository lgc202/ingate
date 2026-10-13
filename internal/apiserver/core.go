package apiserver

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	coreregistry "github.com/lgc202/ingate/internal/apiserver/registry/core"
)

func installCoreAPIGroup(
	server *genericapiserver.GenericAPIServer,
	scheme *runtime.Scheme,
	optionsGetter generic.RESTOptionsGetter,
) error {
	storage, err := coreregistry.NewStorage(scheme, optionsGetter)
	if err != nil {
		return fmt.Errorf("create core API group storage: %w", err)
	}

	apiGroupInfo := coreAPIGroupInfo(scheme, storage)
	if err := server.InstallLegacyAPIGroup(genericapiserver.DefaultLegacyAPIPrefix, &apiGroupInfo); err != nil {
		for _, resourceStorage := range storage {
			resourceStorage.Destroy()
		}

		return fmt.Errorf("install core API group: %w", err)
	}

	return nil
}

func coreAPIGroupInfo(
	scheme *runtime.Scheme,
	storage map[string]rest.Storage,
) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		corev1.GroupName,
		scheme,
		runtime.NewParameterCodec(scheme),
		serializer.NewCodecFactory(scheme),
	)
	apiGroupInfo.VersionedResourcesStorageMap[corev1.SchemeGroupVersion.Version] = storage

	return apiGroupInfo
}
