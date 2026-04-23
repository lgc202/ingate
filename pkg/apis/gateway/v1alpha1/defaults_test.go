package v1alpha1

import "testing"

func TestSetDefaultsBackendDefaultsProtocol(t *testing.T) {
	backend := &Backend{}

	SetDefaults_Backend(backend)

	if backend.Spec.Protocol != BackendProtocolHTTP {
		t.Fatalf("expected protocol %q, got %q", BackendProtocolHTTP, backend.Spec.Protocol)
	}
}

func TestSetDefaultsBackendPreservesExplicitProtocol(t *testing.T) {
	backend := &Backend{
		Spec: BackendSpec{Protocol: BackendProtocolHTTPS},
	}

	SetDefaults_Backend(backend)

	if backend.Spec.Protocol != BackendProtocolHTTPS {
		t.Fatalf("expected protocol %q, got %q", BackendProtocolHTTPS, backend.Spec.Protocol)
	}
}
