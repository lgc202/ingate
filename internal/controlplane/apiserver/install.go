package apiserver

import (
	"fmt"

	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"

	ingatestorage "github.com/lgc202/ingate/pkg/apiserver/storage"
)

func InstallAPIs(
	server *genericapiserver.GenericAPIServer,
	apiResourceConfigSource serverstorage.APIResourceConfigSource,
	restOptionsGetter generic.RESTOptionsGetter,
	providers []ingatestorage.RESTStorageProvider,
) error {
	for _, provider := range providers {
		apiGroupInfo, err := provider.NewRESTStorage(apiResourceConfigSource, restOptionsGetter)
		if err != nil {
			return fmt.Errorf("build storage for group %s: %w", provider.GroupName(), err)
		}
		if err := server.InstallAPIGroup(&apiGroupInfo); err != nil {
			return fmt.Errorf("install group %s: %w", provider.GroupName(), err)
		}
	}
	return nil
}
