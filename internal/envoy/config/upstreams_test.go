package config

import (
	"slices"
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
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
							Type:     gatewayv1.UpstreamTypeApplication,
							Protocol: gatewayv1.UpstreamProtocolHTTP,
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
			if got, want := cluster.Name, "backend"; got != want {
				t.Errorf("Compiler.Compile(endpoint address %q) cluster name = %q, want %q", tt.address, got, want)
			}
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

func TestCompilerBuildsTLSClusterForOpenAIUpstream(t *testing.T) {
	result := (Compiler{}).Compile(ResourceSet{
		Upstreams: []*gatewayv1.Upstream{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "openai"},
				Spec: gatewayv1.UpstreamSpec{
					Type:     gatewayv1.UpstreamTypeModel,
					Protocol: gatewayv1.UpstreamProtocolOpenAI,
					TLS:      &gatewayv1.UpstreamTLS{ServerName: "API.OpenAI.COM"},
					Model: &gatewayv1.ModelSpec{
						Provider:    gatewayv1.ModelProviderOpenAI,
						APIBasePath: "/v1",
						Models: []gatewayv1.ModelCatalogItem{
							{Name: "gpt-4o", DisplayName: "GPT-4o", Enabled: true},
						},
					},
					Endpoints: []gatewayv1.Endpoint{
						{
							Name:    "primary",
							Address: "api.openai.com",
							Port:    443,
							Weight:  100,
							Enabled: true,
						},
					},
				},
			},
		},
	})
	if result.HasErrors() {
		t.Fatalf("Compiler.Compile(OpenAI TLS upstream) diagnostics = %v, want no errors", result.Diagnostics)
	}
	if len(result.Config.Clusters) != 1 {
		t.Fatalf("Compiler.Compile(OpenAI TLS upstream) cluster count = %d, want 1", len(result.Config.Clusters))
	}

	transportSocket := result.Config.Clusters[0].TransportSocket
	if transportSocket == nil {
		t.Fatal("Compiler.Compile(OpenAI TLS upstream) transport socket = nil, want TLS transport socket")
	}
	if got, want := transportSocket.Name, tlsTransportSocketName; got != want {
		t.Errorf("Compiler.Compile(OpenAI TLS upstream) transport socket name = %q, want %q", got, want)
	}
	tlsContext := &tlsv3.UpstreamTlsContext{}
	if err := transportSocket.GetTypedConfig().UnmarshalTo(tlsContext); err != nil {
		t.Fatalf("UnmarshalTo(UpstreamTlsContext) error = %v, want nil", err)
	}
	if got, want := tlsContext.Sni, "api.openai.com"; got != want {
		t.Errorf("UpstreamTlsContext.Sni = %q, want %q", got, want)
	}
	validation := tlsContext.GetCommonTlsContext().GetValidationContext()
	if validation == nil || validation.GetTrustedCa().GetFilename() != systemCABundlePath {
		t.Errorf("UpstreamTlsContext trusted CA = %q, want %q", validation.GetTrustedCa().GetFilename(), systemCABundlePath)
	}
	if got, want := tlsContext.GetCommonTlsContext().GetAlpnProtocols(), []string{"http/1.1"}; !slices.Equal(got, want) {
		t.Errorf("UpstreamTlsContext ALPN protocols = %v, want %v", got, want)
	}
	if validation == nil {
		return
	}
	matchers := validation.GetMatchTypedSubjectAltNames()
	if got, want := len(matchers), 1; got != want {
		t.Fatalf("UpstreamTlsContext subject alternative name matcher count = %d, want %d", got, want)
	}
	if got, want := matchers[0].SanType, tlsv3.SubjectAltNameMatcher_DNS; got != want {
		t.Errorf("UpstreamTlsContext subject alternative name type = %v, want %v", got, want)
	}
	if got, want := matchers[0].GetMatcher().GetExact(), "api.openai.com"; got != want {
		t.Errorf("UpstreamTlsContext exact subject alternative name = %q, want %q", got, want)
	}
}

func TestCompilerOmitsSNIForIPTLSIdentity(t *testing.T) {
	result := (Compiler{}).Compile(ResourceSet{
		Upstreams: []*gatewayv1.Upstream{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "local-model"},
				Spec: gatewayv1.UpstreamSpec{
					Type:     gatewayv1.UpstreamTypeModel,
					Protocol: gatewayv1.UpstreamProtocolOpenAI,
					TLS:      &gatewayv1.UpstreamTLS{ServerName: "192.0.2.10"},
					Model: &gatewayv1.ModelSpec{
						Provider:    gatewayv1.ModelProviderCustom,
						APIBasePath: "/v1",
						Models: []gatewayv1.ModelCatalogItem{
							{Name: "local-model", DisplayName: "Local Model", Enabled: true},
						},
					},
					Endpoints: []gatewayv1.Endpoint{
						{Name: "primary", Address: "192.0.2.10", Port: 443, Weight: 100, Enabled: true},
					},
				},
			},
		},
	})
	if result.HasErrors() {
		t.Fatalf("Compiler.Compile(IP TLS identity) diagnostics = %v, want no errors", result.Diagnostics)
	}

	tlsContext := &tlsv3.UpstreamTlsContext{}
	if err := result.Config.Clusters[0].TransportSocket.GetTypedConfig().UnmarshalTo(tlsContext); err != nil {
		t.Fatalf("UnmarshalTo(UpstreamTlsContext) error = %v, want nil", err)
	}
	if tlsContext.Sni != "" {
		t.Errorf("UpstreamTlsContext.Sni = %q, want empty for IP identity", tlsContext.Sni)
	}
	matchers := tlsContext.GetCommonTlsContext().GetValidationContext().GetMatchTypedSubjectAltNames()
	if got, want := len(matchers), 1; got != want {
		t.Fatalf("UpstreamTlsContext subject alternative name matcher count = %d, want %d", got, want)
	}
	if got, want := matchers[0].SanType, tlsv3.SubjectAltNameMatcher_IP_ADDRESS; got != want {
		t.Errorf("UpstreamTlsContext subject alternative name type = %v, want %v", got, want)
	}
	if got, want := matchers[0].GetMatcher().GetExact(), "192.0.2.10"; got != want {
		t.Errorf("UpstreamTlsContext exact subject alternative name = %q, want %q", got, want)
	}
}

func TestCompilerRejectsUpstreamWithoutType(t *testing.T) {
	upstream := &gatewayv1.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "upstream"},
		Spec: gatewayv1.UpstreamSpec{
			Protocol: gatewayv1.UpstreamProtocolHTTP,
			Endpoints: []gatewayv1.Endpoint{
				{Name: "primary", Address: "192.0.2.10", Port: 8080, Weight: 100, Enabled: true},
			},
		},
	}
	result := (Compiler{}).Compile(ResourceSet{Upstreams: []*gatewayv1.Upstream{upstream}})
	if !result.HasErrors() {
		t.Fatal("Compiler.Compile(upstream without type) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compiler.Compile(upstream without type) diagnostics = %v, want Upstream %q reason %q",
			result.Diagnostics,
			upstream.Name,
			ReasonInvalidSpec,
		)
	}
}
