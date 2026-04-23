package apiserver

import (
	"fmt"

	genericapiserver "k8s.io/apiserver/pkg/server"
)

// IngateAPIServer is the generic apiserver for the Ingate control plane.
type IngateAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

func (c completedConfig) New() (*IngateAPIServer, error) {
	genericServer, err := c.GenericConfig.New("ingate-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, fmt.Errorf("failed to create generic apiserver: %w", err)
	}

	if err := InstallAPIs(genericServer, c.APIResourceConfigSource, c.RESTOptionsGetter, c.RESTStorageProviders); err != nil {
		return nil, fmt.Errorf("failed to install ingate api groups: %w", err)
	}

	return &IngateAPIServer{GenericAPIServer: genericServer}, nil
}
