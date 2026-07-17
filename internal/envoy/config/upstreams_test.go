package config

import (
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestCompilerBuildsClusterForEndpointAddress(t *testing.T) {
	tests := []struct {
		name              string
		address           string
		wantType          clusterv3.Cluster_DiscoveryType
		wantEndpointCount int
		wantInline        bool
	}{
		{
			name:              "IP endpoint uses EDS",
			address:           "127.0.0.1",
			wantType:          clusterv3.Cluster_EDS,
			wantEndpointCount: 1,
		},
		{
			name:       "hostname endpoint uses strict DNS",
			address:    "backend.internal",
			wantType:   clusterv3.Cluster_STRICT_DNS,
			wantInline: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := (Compiler{}).Compile(ResourceSet{
				Upstreams: []*gatewayv1.Upstream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "backend"},
						Spec: gatewayv1.UpstreamSpec{
							Endpoints: []gatewayv1.Endpoint{
								{
									Name:    "primary",
									Address: tt.address,
									Port:    8080,
									Weight:  100,
									Enabled: true,
								},
							},
						},
					},
				},
			})
			if result.HasErrors() {
				t.Fatalf("Compiler.Compile(endpoint address %q) diagnostics = %v, want no errors", tt.address, result.Diagnostics)
			}
			if len(result.Config.Clusters) != 1 {
				t.Fatalf("Compiler.Compile(endpoint address %q) cluster count = %d, want 1", tt.address, len(result.Config.Clusters))
			}

			cluster := result.Config.Clusters[0]
			if got := cluster.GetType(); got != tt.wantType {
				t.Errorf("Compiler.Compile(endpoint address %q) cluster type = %v, want %v", tt.address, got, tt.wantType)
			}
			if got := len(result.Config.Endpoints); got != tt.wantEndpointCount {
				t.Errorf("Compiler.Compile(endpoint address %q) EDS resource count = %d, want %d", tt.address, got, tt.wantEndpointCount)
			}
			if got := cluster.GetLoadAssignment() != nil; got != tt.wantInline {
				t.Errorf("Compiler.Compile(endpoint address %q) inline load assignment = %t, want %t", tt.address, got, tt.wantInline)
			}
		})
	}
}
