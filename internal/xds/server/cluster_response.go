package server

import (
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

func (b responseBuilder) buildClusters(configs []snapshotConfig) ([]*anypb.Any, error) {
	resources := make([]*anypb.Any, 0)
	for _, config := range configs {
		for _, cluster := range config.Config.Clusters {
			resource, err := anypb.New(&clusterv3.Cluster{
				Name: cluster.Name,
				ClusterDiscoveryType: &clusterv3.Cluster_Type{
					Type: clusterv3.Cluster_EDS,
				},
				EdsClusterConfig: &clusterv3.Cluster_EdsClusterConfig{
					EdsConfig:   b.adsConfigSource(),
					ServiceName: cluster.Name,
				},
				ConnectTimeout: durationpb.New(5 * time.Second),
				LbPolicy:       clusterv3.Cluster_ROUND_ROBIN,
			})
			if err != nil {
				return nil, err
			}
			resources = append(resources, resource)
		}
	}
	return resources, nil
}
