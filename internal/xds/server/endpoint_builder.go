package server

import (
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"google.golang.org/protobuf/types/known/anypb"
)

func (b responseBuilder) buildEndpointAssignments(configs []snapshotConfig) ([]*anypb.Any, error) {
	resources := make([]*anypb.Any, 0)
	seen := map[string]struct{}{}
	for _, config := range configs {
		for _, assignment := range config.Config.EndpointAssignments {
			if _, ok := seen[assignment.ClusterName]; ok {
				continue
			}
			seen[assignment.ClusterName] = struct{}{}
			endpoints := make([]*endpointv3.LbEndpoint, 0, len(assignment.Endpoints))
			for _, endpoint := range assignment.Endpoints {
				endpoints = append(endpoints, &endpointv3.LbEndpoint{
					HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
						Endpoint: &endpointv3.Endpoint{
							Address: b.socketAddress(endpoint.Address, endpoint.Port),
						},
					},
				})
			}

			resource, err := anypb.New(&endpointv3.ClusterLoadAssignment{
				ClusterName: assignment.ClusterName,
				Endpoints: []*endpointv3.LocalityLbEndpoints{
					{LbEndpoints: endpoints},
				},
			})
			if err != nil {
				return nil, err
			}
			resources = append(resources, resource)
		}
	}
	return resources, nil
}
