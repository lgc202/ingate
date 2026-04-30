package server

import (
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

const upstreamTLSTransportSocketName = "envoy.transport_sockets.tls"

func (b responseBuilder) buildClusters(configs []snapshotConfig) ([]*anypb.Any, error) {
	resources := make([]*anypb.Any, 0)
	for _, config := range configs {
		for _, cluster := range config.Config.Clusters {
			envoyCluster, err := b.buildCluster(cluster)
			if err != nil {
				return nil, err
			}
			resource, err := anypb.New(envoyCluster)
			if err != nil {
				return nil, err
			}
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

func (b responseBuilder) buildCluster(cluster targetxds.Cluster) (*clusterv3.Cluster, error) {
	envoyCluster := &clusterv3.Cluster{
		Name:           cluster.Name,
		ConnectTimeout: durationpb.New(5 * time.Second),
		LbPolicy:       clusterv3.Cluster_ROUND_ROBIN,
	}

	switch cluster.DiscoveryType {
	case targetxds.ClusterDiscoveryTypeLogicalDNS:
		// AIProvider cluster 使用 LOGICAL_DNS，Envoy 直接解析模型供应商域名
		// 普通 Upstream 仍然走 EDS，由控制面维护端点列表
		envoyCluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_LOGICAL_DNS}
		envoyCluster.LoadAssignment = &endpointv3.ClusterLoadAssignment{
			ClusterName: cluster.Name,
			Endpoints: []*endpointv3.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpointv3.LbEndpoint{
						{
							HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
								Endpoint: &endpointv3.Endpoint{
									Address: b.socketAddress(cluster.Address, cluster.Port),
								},
							},
						},
					},
				},
			},
		}
		if cluster.TLS {
			// AIProvider endpoint 是 https 时必须配置 upstream TLS
			// 否则 Envoy 会用明文 HTTP 连接供应商 443 端口
			tlsContext, err := anypb.New(&tlsv3.UpstreamTlsContext{
				Sni: cluster.Address,
			})
			if err != nil {
				return nil, err
			}
			envoyCluster.TransportSocket = &corev3.TransportSocket{
				Name: upstreamTLSTransportSocketName,
				ConfigType: &corev3.TransportSocket_TypedConfig{
					TypedConfig: tlsContext,
				},
			}
		}
	default:
		envoyCluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_EDS}
		envoyCluster.EdsClusterConfig = &clusterv3.Cluster_EdsClusterConfig{
			EdsConfig:   b.adsConfigSource(),
			ServiceName: cluster.Name,
		}
	}

	return envoyCluster, nil
}
