package server

import (
	"fmt"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
)

const (
	listenerTypeURL = "type.googleapis.com/envoy.config.listener.v3.Listener"
)

type responseBuilder struct {
	store *snapshotStore
}

func newResponseBuilder(store *snapshotStore) responseBuilder {
	return responseBuilder{store: store}
}

func (b responseBuilder) Build(request *discoveryv3.DiscoveryRequest) (*discoveryv3.DiscoveryResponse, bool, error) {
	switch request.GetTypeUrl() {
	case listenerTypeURL:
		return b.buildEmptyResponse(request), true, nil
	case "":
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("unsupported xds type %q", request.GetTypeUrl())
	}
}

func (b responseBuilder) buildEmptyResponse(request *discoveryv3.DiscoveryRequest) *discoveryv3.DiscoveryResponse {
	return &discoveryv3.DiscoveryResponse{
		VersionInfo: "empty",
		TypeUrl:     request.GetTypeUrl(),
		Nonce:       "empty",
	}
}
