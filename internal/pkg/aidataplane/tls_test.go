package aidataplane

import "testing"

func TestTLSRevisionAndTargets(t *testing.T) {
	config, _ := loadFixture(t)
	backend, network := config.Backends[0], config.NetworkBackends[0]
	previousModel, err := ModelTargetName(backend, network)
	if err != nil {
		t.Fatalf("ModelTargetName(plaintext) error = %v, want nil", err)
	}

	previousMCP, err := MCPTargetName(network)
	if err != nil {
		t.Fatalf("MCPTargetName(plaintext) error = %v, want nil", err)
	}

	previousRevision, err := contentRevision(config)
	if err != nil {
		t.Fatalf("contentRevision(plaintext) error = %v, want nil", err)
	}

	for _, identity := range []string{"service.example.com", "other.example.com"} {
		network.TLS = &BackendTLSSettings{SNI: identity, WellKnownCACertificates: "System"}
		config.NetworkBackends[0] = network

		data, err := Encode(config)
		if err != nil {
			t.Fatalf("Encode(TLS=%q) error = %v", identity, err)
		}

		decoded, err := Decode(data)
		if err != nil {
			t.Fatalf("Decode(TLS=%q) error = %v, want nil", identity, err)
		}

		if tls := decoded.NetworkBackends[0].TLS; tls == nil || tls.SNI != identity {
			t.Fatalf("Decode(TLS=%q) TLS = %+v, want identity preserved", identity, tls)
		}

		if decoded.Revision == previousRevision {
			t.Errorf("Decode(TLS=%q) revision = %q, want changed revision", identity, decoded.Revision)
		}

		model, err := ModelTargetName(backend, network)
		if err != nil || model == previousModel {
			t.Errorf("ModelTargetName(TLS=%q) = (%q, %v), want changed target", identity, model, err)
		}

		mcp, err := MCPTargetName(network)
		if err != nil || mcp == previousMCP {
			t.Errorf("MCPTargetName(TLS=%q) = (%q, %v), want changed target", identity, mcp, err)
		}

		previousRevision = decoded.Revision
		previousModel, previousMCP = model, mcp
	}

	for _, tls := range []BackendTLSSettings{
		{},
		{SNI: "192.0.2.1", WellKnownCACertificates: "System"},
		{SNI: "service.example.com", WellKnownCACertificates: "Private"},
	} {
		config.NetworkBackends[0].TLS = &tls

		data, err := Encode(config)
		if err == nil || data != nil {
			t.Errorf("Encode(TLS=%+v) = (%s, %v), want no output and error", tls, data, err)
		}
	}
}
