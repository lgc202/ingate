package server

import (
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
)

func TestResponseBuilderBuildClustersWithLogicalDNSAndTLS(t *testing.T) {
	configs := []snapshotConfig{
		{
			Gateway: "public",
			Version: "xds/public",
			Config: targetxds.Config{
				Clusters: []targetxds.Cluster{
					{
						Name:          "ai-provider/openai",
						DiscoveryType: targetxds.ClusterDiscoveryTypeLogicalDNS,
						Address:       "api.openai.com",
						Port:          443,
						TLS:           true,
					},
				},
			},
		},
	}

	resources, err := (responseBuilder{}).buildClusters(configs)
	if err != nil {
		t.Fatalf("buildClusters() error = %v", err)
	}

	var cluster clusterv3.Cluster
	if err := resources[0].UnmarshalTo(&cluster); err != nil {
		t.Fatalf("UnmarshalTo(cluster) error = %v", err)
	}
	if cluster.GetType() != clusterv3.Cluster_LOGICAL_DNS {
		t.Fatalf("Cluster type = %v, want LOGICAL_DNS", cluster.GetType())
	}
	address := cluster.GetLoadAssignment().GetEndpoints()[0].GetLbEndpoints()[0].GetEndpoint().GetAddress().GetSocketAddress().GetAddress()
	if address != "api.openai.com" {
		t.Fatalf("Cluster address = %q, want api.openai.com", address)
	}
	if cluster.GetTransportSocket().GetName() != upstreamTLSTransportSocketName {
		t.Fatalf("TransportSocket name = %q, want TLS", cluster.GetTransportSocket().GetName())
	}

	var tlsContext tlsv3.UpstreamTlsContext
	if err := cluster.GetTransportSocket().GetTypedConfig().UnmarshalTo(&tlsContext); err != nil {
		t.Fatalf("UnmarshalTo(tlsContext) error = %v", err)
	}
	if tlsContext.GetSni() != "api.openai.com" {
		t.Fatalf("TLS SNI = %q, want api.openai.com", tlsContext.GetSni())
	}
}
