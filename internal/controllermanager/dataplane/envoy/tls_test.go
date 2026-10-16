package envoy

import (
	"strings"
	"testing"

	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestNewClusterTLS(t *testing.T) {
	backend := dataplane.Backend{
		Name:      "service",
		Endpoints: []dataplane.Endpoint{{Address: "2001:db8::1", Port: 8443, Weight: 1}},
		TLS:       &dataplane.BackendTLSSettings{SNI: "service.example.com"},
	}

	cluster, assignment, err := newCluster(backend)
	if err != nil {
		t.Fatalf("newCluster(TLS) error = %v, want nil", err)
	}

	var tlsContext tlsv3.UpstreamTlsContext
	if err := cluster.TransportSocket.GetTypedConfig().UnmarshalTo(&tlsContext); err != nil {
		t.Fatalf("UnmarshalTo(upstream TLS) error = %v, want nil", err)
	}

	validation := tlsContext.CommonTlsContext.GetValidationContext()
	if tlsContext.Sni != "service.example.com" || validation.GetTrustedCa().GetFilename() != "/etc/ssl/certs/ca-certificates.crt" {
		t.Errorf("newCluster(TLS) identity/CA = %v, want fixed DNS and system bundle", &tlsContext)
	}

	matchers := validation.GetMatchTypedSubjectAltNames()
	if len(matchers) != 1 || matchers[0].SanType != tlsv3.SubjectAltNameMatcher_DNS || matchers[0].Matcher.GetExact() != tlsContext.Sni {
		t.Errorf("newCluster(TLS) SAN matchers = %v, want same DNS identity as SNI", matchers)
	}

	address := assignment.Endpoints[0].LbEndpoints[0].GetEndpoint().GetAddress().GetSocketAddress()
	if address.Address != "2001:db8::1" || address.GetPortValue() != 8443 {
		t.Errorf("newCluster(TLS) address = %v, want original IPv6 and explicit 8443", address)
	}

	// 领域身份由输入边界校验；此处核对 Envoy 配置构建失败仍不返回明文结果。
	backend.TLS.SNI = strings.Repeat("a", 256)
	cluster, assignment, err = newCluster(backend)
	if err == nil || cluster != nil || assignment != nil {
		t.Errorf("newCluster(invalid TLS) = (%v, %v, %v), want nil results and error without plaintext fallback", cluster, assignment, err)
	}

	backend.TLS = nil
	cluster, _, err = newCluster(backend)
	if err != nil || cluster.GetTransportSocket() != nil {
		t.Errorf("newCluster(removed TLS) = (%v, %v), want plaintext cluster", cluster, err)
	}
}

func TestBuildAIResourcesTLS(t *testing.T) {
	config := aiFixture(t)
	for i := range config.NetworkBackends {
		config.NetworkBackends[i].TLS = &aidataplane.BackendTLSSettings{SNI: "service.example.com", WellKnownCACertificates: "System"}
	}

	resources, err := buildAIResources(config, "/run/ingate/ingress.sock", "/run/ingate/egress.sock")
	if err != nil {
		t.Fatalf("buildAIResources(TLS) error = %v, want nil", err)
	}

	var encrypted int
	for _, cluster := range resources.clusters {
		if cluster.TransportSocket == nil {
			continue
		}

		var tlsContext tlsv3.UpstreamTlsContext
		if err := cluster.TransportSocket.GetTypedConfig().UnmarshalTo(&tlsContext); err != nil {
			t.Fatalf("UnmarshalTo(%q TLS) error = %v", cluster.Name, err)
		}

		if tlsContext.Sni != "service.example.com" {
			t.Errorf("buildAIResources(TLS) cluster %q SNI = %q, want configured identity", cluster.Name, tlsContext.Sni)
		}

		encrypted++
	}

	if encrypted != 2 {
		t.Errorf("buildAIResources(TLS) encrypted clusters = %d, want model and MCP", encrypted)
	}
}
