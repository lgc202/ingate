package compiler

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestCompilerBuildsSharedHTTPSListenerWithGatewayCertificates(t *testing.T) {
	apiCertificate := newTestCertificate(t, "api-certificate", "api.example.com")
	mcpCertificate := newTestCertificate(t, "mcp-certificate", "mcp.example.com")
	result := Compile(Resources{
		Certificates: []*gatewayv1.Certificate{apiCertificate, mcpCertificate},
		Gateways: []*gatewayv1.Gateway{
			newHTTPSGateway("api-gateway", "api.example.com", apiCertificate.Name),
			newHTTPSGateway("mcp-gateway", "mcp.example.com", mcpCertificate.Name),
		},
	})
	if result.HasErrors() {
		t.Fatalf("Compile(shared HTTPS listener) diagnostics = %v, want no errors", result.Diagnostics)
	}
	if got := len(result.Config.Listeners); got != 1 {
		t.Fatalf("Compile(shared HTTPS listener) listener count = %d, want 1", got)
	}

	listener := result.Config.Listeners[0]
	if got, want := listener.Name, "ingate/https-8443"; got != want {
		t.Errorf("Compile(shared HTTPS listener) listener name = %q, want %q", got, want)
	}
	if got := len(listener.ListenerFilters); got != 1 {
		t.Errorf("Compile(shared HTTPS listener) listener filter count = %d, want 1", got)
	} else if got, want := listener.ListenerFilters[0].Name, tlsInspectorFilterName; got != want {
		t.Errorf("Compile(shared HTTPS listener) listener filter name = %q, want %q", got, want)
	}
	if listener.DefaultFilterChain != nil {
		t.Errorf("Compile(shared HTTPS listener) default filter chain = %v, want nil", listener.DefaultFilterChain)
	}
	if got := len(listener.FilterChains); got != 2 {
		t.Fatalf("Compile(shared HTTPS listener) filter chain count = %d, want 2", got)
	}

	wantCertificates := map[string]*gatewayv1.Certificate{
		"api.example.com": apiCertificate,
		"mcp.example.com": mcpCertificate,
	}
	for _, filterChain := range listener.FilterChains {
		if filterChain.FilterChainMatch == nil || len(filterChain.FilterChainMatch.ServerNames) != 1 {
			t.Errorf("Compile(shared HTTPS listener) filter chain match = %v, want one SNI server name", filterChain.FilterChainMatch)
			continue
		}
		hostname := filterChain.FilterChainMatch.ServerNames[0]
		wantCertificate := wantCertificates[hostname]
		if wantCertificate == nil {
			t.Errorf("Compile(shared HTTPS listener) SNI hostname = %q, want api.example.com or mcp.example.com", hostname)
			continue
		}
		if got, want := filterChain.FilterChainMatch.TransportProtocol, tlsTransportProtocol; got != want {
			t.Errorf("Compile(shared HTTPS listener) transport protocol for %q = %q, want %q", hostname, got, want)
		}
		if filterChain.TransportSocket == nil {
			t.Errorf("Compile(shared HTTPS listener) transport socket for %q = nil, want TLS", hostname)
			continue
		}
		if got, want := filterChain.TransportSocket.Name, tlsTransportSocketName; got != want {
			t.Errorf("Compile(shared HTTPS listener) transport socket for %q = %q, want %q", hostname, got, want)
		}

		tlsContext := &tlsv3.DownstreamTlsContext{}
		if err := filterChain.TransportSocket.GetTypedConfig().UnmarshalTo(tlsContext); err != nil {
			t.Fatalf("Compile(shared HTTPS listener) TLS context for %q could not be decoded: %v", hostname, err)
		}
		tlsCertificates := tlsContext.GetCommonTlsContext().GetTlsCertificates()
		if got := len(tlsCertificates); got != 1 {
			t.Fatalf("Compile(shared HTTPS listener) TLS certificate count for %q = %d, want 1", hostname, got)
		}
		if got, want := tlsCertificates[0].GetCertificateChain().GetInlineString(), wantCertificate.Spec.CertificatePEM; got != want {
			t.Errorf("Compile(shared HTTPS listener) certificate PEM for %q does not match referenced certificate", hostname)
		}
		if got, want := tlsCertificates[0].GetPrivateKey().GetInlineString(), wantCertificate.Spec.PrivateKeyPEM; got != want {
			t.Errorf("Compile(shared HTTPS listener) private key PEM for %q does not match referenced certificate", hostname)
		}
		if got := len(filterChain.Filters); got != 1 {
			t.Fatalf("Compile(shared HTTPS listener) network filter count for %q = %d, want 1", hostname, got)
		}
		manager := &hcmv3.HttpConnectionManager{}
		if err := filterChain.Filters[0].GetTypedConfig().UnmarshalTo(manager); err != nil {
			t.Fatalf("Compile(shared HTTPS listener) HCM for %q could not be decoded: %v", hostname, err)
		}
		if got, want := manager.GetRds().GetRouteConfigName(), "ingate/https-8443/routes"; got != want {
			t.Errorf("Compile(shared HTTPS listener) RDS name for %q = %q, want %q", hostname, got, want)
		}
	}
}

func TestCompilerBuildsDefaultHTTPSFilterChainForCatchAllGateway(t *testing.T) {
	certificate := newTestCertificate(t, "default-certificate", "default.example.com")
	gateway := newHTTPSGateway("default-gateway", "", certificate.Name)
	gateway.Spec.HostBindings = nil

	result := Compile(Resources{
		Certificates: []*gatewayv1.Certificate{certificate},
		Gateways:     []*gatewayv1.Gateway{gateway},
	})
	if result.HasErrors() {
		t.Fatalf("Compile(catch-all HTTPS listener) diagnostics = %v, want no errors", result.Diagnostics)
	}
	if got := len(result.Config.Listeners); got != 1 {
		t.Fatalf("Compile(catch-all HTTPS listener) listener count = %d, want 1", got)
	}

	listener := result.Config.Listeners[0]
	if got := len(listener.FilterChains); got != 0 {
		t.Errorf("Compile(catch-all HTTPS listener) matched filter chain count = %d, want 0", got)
	}
	if listener.DefaultFilterChain == nil {
		t.Fatal("Compile(catch-all HTTPS listener) default filter chain = nil, want TLS filter chain")
	}
	if listener.DefaultFilterChain.FilterChainMatch != nil {
		t.Errorf("Compile(catch-all HTTPS listener) default filter chain match = %v, want nil", listener.DefaultFilterChain.FilterChainMatch)
	}
}

func TestCompilerReportsInvalidHTTPSCertificateReferences(t *testing.T) {
	invalidCertificate := &gatewayv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "invalid-certificate"},
		Spec: gatewayv1.CertificateSpec{
			CertificatePEM: "invalid certificate",
			PrivateKeyPEM:  "invalid private key",
		},
	}
	missingReference := newHTTPSGateway("missing-reference", "missing.example.com", "")
	unknownReference := newHTTPSGateway("unknown-reference", "unknown.example.com", "unknown-certificate")
	invalidReference := newHTTPSGateway("invalid-reference", "invalid.example.com", invalidCertificate.Name)

	result := Compile(Resources{
		Certificates: []*gatewayv1.Certificate{invalidCertificate},
		Gateways: []*gatewayv1.Gateway{
			missingReference,
			unknownReference,
			invalidReference,
		},
	})
	if !result.HasErrors() {
		t.Fatal("Compile(invalid HTTPS certificate references) has errors = false, want true")
	}

	wants := []Diagnostic{
		{Kind: gatewayv1.KindCertificate, ID: invalidCertificate.Name, Reason: ReasonInvalidSpec},
		{Kind: gatewayv1.KindGateway, ID: missingReference.Name, Reason: ReasonInvalidSpec},
		{Kind: gatewayv1.KindGateway, ID: unknownReference.Name, Reason: ReasonReferenceNotFound},
		{Kind: gatewayv1.KindGateway, ID: invalidReference.Name, Reason: ReasonInvalidSpec},
	}
	for _, want := range wants {
		if !containsDiagnostic(result.Diagnostics, want.Kind, want.ID, want.Reason) {
			t.Errorf(
				"Compile(invalid HTTPS certificate references) diagnostics = %v, want kind %q id %q reason %q",
				result.Diagnostics,
				want.Kind,
				want.ID,
				want.Reason,
			)
		}
	}
}

func TestCompilerReportsConflictingHostOwnership(t *testing.T) {
	tests := []struct {
		name       string
		protocol   gatewayv1.Protocol
		port       int
		firstHost  string
		secondHost string
	}{
		{
			name:       "HTTP catch all and exact hostname",
			protocol:   gatewayv1.ProtocolHTTP,
			port:       8080,
			secondHost: "api.example.com",
		},
		{
			name:       "HTTP same exact hostname",
			protocol:   gatewayv1.ProtocolHTTP,
			port:       8080,
			firstHost:  "api.example.com",
			secondHost: "api.example.com",
		},
		{
			name:       "HTTPS wildcard and exact hostname",
			protocol:   gatewayv1.ProtocolHTTPS,
			port:       8443,
			firstHost:  "*.example.com",
			secondHost: "api.example.com",
		},
		{
			name:       "HTTPS nested wildcards",
			protocol:   gatewayv1.ProtocolHTTPS,
			port:       8443,
			firstHost:  "*.example.com",
			secondHost: "*.api.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var certificates []*gatewayv1.Certificate
			firstCertificateRef := ""
			secondCertificateRef := ""
			if tt.protocol == gatewayv1.ProtocolHTTPS {
				firstCertificate := newTestCertificate(t, "first-certificate", "first.example.com")
				secondCertificate := newTestCertificate(t, "second-certificate", "second.example.com")
				certificates = []*gatewayv1.Certificate{firstCertificate, secondCertificate}
				firstCertificateRef = firstCertificate.Name
				secondCertificateRef = secondCertificate.Name
			}

			firstGateway := newTestGateway("first-gateway", tt.protocol, tt.port, tt.firstHost, firstCertificateRef)
			secondGateway := newTestGateway("second-gateway", tt.protocol, tt.port, tt.secondHost, secondCertificateRef)
			result := Compile(Resources{
				Certificates: certificates,
				Gateways:     []*gatewayv1.Gateway{firstGateway, secondGateway},
			})
			if !result.HasErrors() {
				t.Fatalf("Compile(%q) has errors = false, want true", tt.name)
			}
			for _, gateway := range []*gatewayv1.Gateway{firstGateway, secondGateway} {
				if !containsDiagnostic(result.Diagnostics, gatewayv1.KindGateway, gateway.Name, ReasonConflict) {
					t.Errorf(
						"Compile(%q) diagnostics = %v, want conflict for gateway %q",
						tt.name,
						result.Diagnostics,
						gateway.Name,
					)
				}
			}
		})
	}
}

func TestCompilerReportsDifferentProtocolsOnSamePort(t *testing.T) {
	certificate := newTestCertificate(t, "https-certificate", "mcp.example.com")
	httpGateway := newTestGateway("http-gateway", gatewayv1.ProtocolHTTP, 8443, "api.example.com", "")
	httpsGateway := newTestGateway("https-gateway", gatewayv1.ProtocolHTTPS, 8443, "mcp.example.com", certificate.Name)

	result := Compile(Resources{
		Certificates: []*gatewayv1.Certificate{certificate},
		Gateways:     []*gatewayv1.Gateway{httpGateway, httpsGateway},
	})
	if !result.HasErrors() {
		t.Fatal("Compile(different protocols on same port) has errors = false, want true")
	}
	for _, gateway := range []*gatewayv1.Gateway{httpGateway, httpsGateway} {
		if !containsDiagnostic(result.Diagnostics, gatewayv1.KindGateway, gateway.Name, ReasonConflict) {
			t.Errorf(
				"Compile(different protocols on same port) diagnostics = %v, want conflict for gateway %q",
				result.Diagnostics,
				gateway.Name,
			)
		}
	}
}

func TestCompilerAllowsSameHostnameOnHTTPAndHTTPSDefaultPorts(t *testing.T) {
	certificate := newTestCertificate(t, "https-certificate", "api.example.com")
	httpGateway := newTestGateway("http-gateway", gatewayv1.ProtocolHTTP, 8080, "api.example.com", "")
	httpsGateway := newTestGateway("https-gateway", gatewayv1.ProtocolHTTPS, 8443, "api.example.com", certificate.Name)

	result := Compile(Resources{
		Certificates: []*gatewayv1.Certificate{certificate},
		Gateways:     []*gatewayv1.Gateway{httpGateway, httpsGateway},
	})
	if result.HasErrors() {
		t.Fatalf("Compile(same hostname on HTTP and HTTPS default ports) diagnostics = %v, want no errors", result.Diagnostics)
	}
	if got := len(result.Config.Listeners); got != 2 {
		t.Errorf("Compile(same hostname on HTTP and HTTPS default ports) listener count = %d, want 2", got)
	}
}

func newHTTPSGateway(id, hostname, certificateRef string) *gatewayv1.Gateway {
	return newTestGateway(id, gatewayv1.ProtocolHTTPS, 8443, hostname, certificateRef)
}

func newTestGateway(id string, protocol gatewayv1.Protocol, port int, hostname, certificateRef string) *gatewayv1.Gateway {
	listenerName := strings.ToLower(string(protocol))
	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: id},
		Spec: gatewayv1.GatewaySpec{
			Enabled: true,
			Listeners: []gatewayv1.Listener{
				{
					Name:           listenerName,
					Protocol:       protocol,
					Port:           port,
					CertificateRef: certificateRef,
				},
			},
		},
	}
	if hostname != "" {
		gateway.Spec.HostBindings = []gatewayv1.HostBinding{
			{
				Hostname:     hostname,
				ListenerRefs: []string{listenerName},
			},
		}
	}
	return gateway
}

func newTestCertificate(t *testing.T, id, hostname string) *gatewayv1.Certificate {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: hostname},
		DNSNames:     []string{hostname},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(%q) error: %v", hostname, err)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey(%q) error: %v", hostname, err)
	}

	return &gatewayv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: id},
		Spec: gatewayv1.CertificateSpec{
			CertificatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})),
			PrivateKeyPEM:  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})),
		},
	}
}

func containsDiagnostic(diagnostics []Diagnostic, kind gatewayv1.Kind, id string, reason Reason) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind && diagnostic.ID == id && diagnostic.Reason == reason {
			return true
		}
	}
	return false
}
