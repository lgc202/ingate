package aidataplane

import (
	"slices"
	"testing"
)

func TestModelTargetName(t *testing.T) {
	config, _ := loadFixture(t)
	backend, network := config.Backends[0], config.NetworkBackends[0]
	const want = "ai/target/96914ad662f712bc8376b8a6ec1aee8de3bb4f51dcd55f0670220c8fbe183d07"

	name, err := ModelTargetName(backend, network)

	if err != nil || name != want {
		t.Fatalf("ModelTargetName(golden) = (%q, %v), want (%q, nil)", name, err, want)
	}

	for _, tt := range []struct {
		name   string
		change func(*Backend, *NetworkBackend)
	}{
		{name: "protocol binding", change: func(b *Backend, _ *NetworkBackend) { b.Name += "-other" }},
		{name: "prefix", change: func(b *Backend, _ *NetworkBackend) { b.Schema.Prefix = "/v2" }},
		{name: "model override", change: func(b *Backend, _ *NetworkBackend) { b.ModelNameOverride = new("other") }},
		{name: "address", change: func(_ *Backend, n *NetworkBackend) { n.Endpoints[0].Address = "other.example.com" }},
		{name: "port", change: func(_ *Backend, n *NetworkBackend) { n.Endpoints[0].Port++ }},
		{name: "weight", change: func(_ *Backend, n *NetworkBackend) { n.Endpoints[0].Weight++ }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, n := backend, network
			n.Endpoints = slices.Clone(network.Endpoints)
			tt.change(&b, &n)

			changed, err := ModelTargetName(b, n)

			if err != nil || changed == name {
				t.Errorf("ModelTargetName(changed %s) = (%q, %v), want new name", tt.name, changed, err)
			}
		})
	}
}

func TestModelTargetNameOrderAndReferences(t *testing.T) {
	config, _ := loadFixture(t)
	backend, network := config.Backends[0], config.NetworkBackends[0]
	network.Endpoints = append(network.Endpoints, Endpoint{Address: "192.0.2.1", Port: 8081, Weight: 2})
	before := slices.Clone(network.Endpoints)
	first, err := ModelTargetName(backend, network)
	if err != nil {
		t.Fatalf("ModelTargetName(first) error = %v", err)
	}

	if !slices.Equal(before, network.Endpoints) {
		t.Error("ModelTargetName(first) mutated source endpoint order")
	}

	slices.Reverse(network.Endpoints)

	second, err := ModelTargetName(backend, network)

	if err != nil || second != first {
		t.Errorf("ModelTargetName(reversed) = (%q, %v), want (%q, nil)", second, err, first)
	}

	network.Name = "team-b/model-network"

	name, err := ModelTargetName(backend, network)

	if err == nil || name != "" {
		t.Errorf("ModelTargetName(wrong network) = (%q, %v), want empty name and error", name, err)
	}
}
