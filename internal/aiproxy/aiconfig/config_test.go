package aiconfig

import (
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestNewConfig(t *testing.T) {
	data, err := os.ReadFile("../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(configuration fixture) error = %v", err)
	}
	want, err := aidataplane.Decode(data)
	if err != nil {
		t.Fatalf("Decode(configuration fixture) error = %v", err)
	}

	config, err := newConfig(data)
	if err != nil {
		t.Fatalf("newConfig(configuration fixture) error = %v", err)
	}

	if diff := cmp.Diff(want, config.config); diff != "" {
		t.Errorf("newConfig() changed compiled contents (-want +got):\n%s", diff)
	}
	if config.Revision() != want.Revision {
		t.Errorf("Revision() = %q, want %q", config.Revision(), want.Revision)
	}

	for i := range data {
		data[i] = 'x'
	}

	if diff := cmp.Diff(want, config.config); diff != "" {
		t.Errorf("newConfig() retained mutable source bytes (-want +got):\n%s", diff)
	}
}

func TestNewConfigRejectsInvalidPayload(t *testing.T) {
	for _, data := range []string{"", "{", "{}", "null"} {
		config, err := newConfig([]byte(data))
		if err == nil || config != nil {
			t.Errorf("newConfig(%q) = (%+v, %v), want nil configuration and error", data, config, err)
		}
	}

	data, err := aidataplane.Encode(configFixture(t))
	if err != nil {
		t.Fatalf("Encode(configuration fixture) error = %v", err)
	}
	compiled, err := aidataplane.Decode(data)
	if err != nil {
		t.Fatalf("Decode(encoded configuration) error = %v", err)
	}
	invalid := strings.Replace(string(data), compiled.Revision, strings.Repeat("z", 64), 1)

	config, err := newConfig([]byte(invalid))
	var invalidByte hex.InvalidByteError
	if config != nil || !errors.As(err, &invalidByte) {
		t.Errorf("newConfig(invalid revision) = (%+v, %v), want nil and wrapped hex.InvalidByteError", config, err)
	}
}

func TestResolveModel(t *testing.T) {
	config := parseConfig(t, configFixture(t))
	want := ModelBackend{
		RouteName:          "team-a/coding/rule/0/match/0",
		Name:               "team-a/model-service/route/coding/rule/0/ref/0",
		Schema:             aidataplane.APISchema{Name: aidataplane.APISchemaOpenAI, Prefix: "/v1"},
		ModelNameOverride:  "code-v2",
		NetworkBackendName: "team-a/model-network",
	}

	got, err := config.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding")
	if err != nil {
		t.Fatalf("ResolveModel(team-a, coding) error = %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ResolveModel(team-a, coding) mismatch (-want +got):\n%s", diff)
	}

	got.Schema.Prefix = "/changed"
	got.ModelNameOverride = "changed"
	again, err := config.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding")
	if err != nil {
		t.Fatalf("ResolveModel(after result mutation) error = %v", err)
	}

	if diff := cmp.Diff(want, again); diff != "" {
		t.Errorf("ResolveModel() returned mutable shared state (-want +got):\n%s", diff)
	}
}

func TestResolveModelDoesNotCrossScope(t *testing.T) {
	config := parseConfig(t, configFixture(t))
	for _, tt := range []struct {
		name  string
		port  int32
		host  string
		model string
	}{
		{name: "unknown model", port: 8080, host: "gateway/team-a/public/listener/api", model: "other"},
		{name: "model case", port: 8080, host: "gateway/team-a/public/listener/api", model: "Coding"},
		{name: "model whitespace", port: 8080, host: "gateway/team-a/public/listener/api", model: " coding"},
		{name: "other namespace", port: 8080, host: "gateway/team-b/public/listener/api", model: "coding"},
		{name: "other listener", port: 8080, host: "gateway/team-a/public/listener/other", model: "coding"},
		{name: "other port", port: 8081, host: "gateway/team-a/public/listener/api", model: "coding"},
		{name: "public hostname", port: 8080, host: "gateway.example.com", model: "coding"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.ResolveModel(tt.port, tt.host, tt.model)

			if !errors.Is(err, ErrModelRouteNotFound) || got != (ModelBackend{}) {
				t.Errorf("ResolveModel(%d, %q, %q) = (%+v, %v), want zero backend and ErrModelRouteNotFound",
					tt.port, tt.host, tt.model, got, err)
			}
		})
	}
}

func TestResolveModelPreservesOrderAndBindings(t *testing.T) {
	compiled := configFixture(t)
	second := compiled.Backends[0]
	second.Name = "team-a/model-service/route/coding/rule/1/ref/0"
	second.Schema.Prefix = "v2/"
	second.ModelNameOverride = new("")
	compiled.Backends = append(compiled.Backends, second)
	first := compiled.Listeners[0].Hosts[0].ModelRoutes[0]
	fallback := aidataplane.ModelRoute{
		Name:        "team-a/coding/rule/1/match/0",
		Headers:     []aidataplane.HeaderMatch{},
		BackendName: new(second.Name),
	}
	compiled.Listeners[0].Hosts[0].ModelRoutes = append(compiled.Listeners[0].Hosts[0].ModelRoutes, fallback)
	config := parseConfig(t, compiled)

	got, err := config.ResolveModel(8080, "gateway/team-a/public/listener/api", "other")
	if err != nil {
		t.Fatalf("ResolveModel(catch-all) error = %v", err)
	}

	want := ModelBackend{
		RouteName:          fallback.Name,
		Name:               second.Name,
		Schema:             second.Schema,
		ModelNameOverride:  "",
		NetworkBackendName: second.NetworkBackendName,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ResolveModel(catch-all) mismatch (-want +got):\n%s", diff)
	}

	compiled.Listeners[0].Hosts[0].ModelRoutes = []aidataplane.ModelRoute{fallback, first}
	config = parseConfig(t, compiled)
	got, err = config.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding")
	if err != nil {
		t.Fatalf("ResolveModel(compiler order) error = %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ResolveModel() reordered compiler candidates (-want +got):\n%s", diff)
	}
}

func TestResolveModelRejectsUnavailableFirstMatch(t *testing.T) {
	compiled := configFixture(t)
	first := compiled.Listeners[0].Hosts[0].ModelRoutes[0]
	first.Name = "team-a/coding/rule/0/match/unavailable"
	first.BackendName = new("")
	compiled.Listeners[0].Hosts[0].ModelRoutes = append([]aidataplane.ModelRoute{first}, compiled.Listeners[0].Hosts[0].ModelRoutes...)
	config := parseConfig(t, compiled)

	got, err := config.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding")

	if !errors.Is(err, ErrModelBackendUnavailable) || got != (ModelBackend{}) {
		t.Errorf("ResolveModel(unavailable before valid match) = (%+v, %v), want zero backend and ErrModelBackendUnavailable", got, err)
	}
}

func TestResolveModelSeparatesPorts(t *testing.T) {
	compiled := configFixture(t)
	other := compiled.Listeners[0].Hosts[0]
	other.ModelRoutes = []aidataplane.ModelRoute{}
	compiled.Listeners = append(compiled.Listeners, aidataplane.Listener{Port: 8081, Hosts: []aidataplane.Host{other}})
	config := parseConfig(t, compiled)

	if _, err := config.ResolveModel(8080, other.Name, "coding"); err != nil {
		t.Errorf("ResolveModel(original port) error = %v, want nil", err)
	}
	if got, err := config.ResolveModel(8081, other.Name, "coding"); !errors.Is(err, ErrModelRouteNotFound) || got != (ModelBackend{}) {
		t.Errorf("ResolveModel(other port) = (%+v, %v), want zero backend and ErrModelRouteNotFound", got, err)
	}
}

func TestResolveModelEmptyConfig(t *testing.T) {
	compiled := aidataplane.Config{
		FormatVersion:   aidataplane.FormatVersionV1,
		Listeners:       []aidataplane.Listener{},
		Backends:        []aidataplane.Backend{},
		NetworkBackends: []aidataplane.NetworkBackend{},
	}
	config := parseConfig(t, compiled)

	got, err := config.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding")

	if !errors.Is(err, ErrModelRouteNotFound) || got != (ModelBackend{}) {
		t.Errorf("ResolveModel(empty configuration) = (%+v, %v), want zero backend and ErrModelRouteNotFound", got, err)
	}
	if diff := cmp.Diff(compiled.Listeners, config.config.Listeners); diff != "" {
		t.Errorf("newConfig(empty) listeners mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveModelDifferentHostBindings(t *testing.T) {
	compiled := configFixture(t)
	otherBackend := compiled.Backends[0]
	otherBackend.Name = "team-b/model-service/route/coding/rule/0/ref/0"
	otherBackend.NetworkBackendName = "team-b/model-network"
	otherBackend.Schema.Prefix = "/v2"
	otherBackend.ModelNameOverride = new("b-code-v2")
	compiled.Backends = append(compiled.Backends, otherBackend)
	otherNetwork := compiled.NetworkBackends[0]
	otherNetwork.Name = otherBackend.NetworkBackendName
	compiled.NetworkBackends = append(compiled.NetworkBackends, otherNetwork)
	otherHost := compiled.Listeners[0].Hosts[0]
	otherHost.Name = "gateway/team-b/public/listener/api"
	otherHost.Hostname = new("b.example.com")
	otherHost.ModelRoutes = []aidataplane.ModelRoute{{
		Name:        "team-b/coding/rule/0/match/0",
		Headers:     otherHost.ModelRoutes[0].Headers,
		BackendName: new(otherBackend.Name),
	}}
	otherHost.MCPRoutes = []aidataplane.MCPRoute{}
	compiled.Listeners[0].Hosts = append(compiled.Listeners[0].Hosts, otherHost)
	config := parseConfig(t, compiled)

	for _, tt := range []struct {
		host    string
		backend aidataplane.Backend
	}{
		{host: "gateway/team-a/public/listener/api", backend: compiled.Backends[0]},
		{host: otherHost.Name, backend: otherBackend},
	} {
		got, err := config.ResolveModel(8080, tt.host, "coding")
		if err != nil {
			t.Fatalf("ResolveModel(%q, coding) error = %v", tt.host, err)
		}

		if got.Name != tt.backend.Name || got.NetworkBackendName != tt.backend.NetworkBackendName ||
			got.Schema != tt.backend.Schema || got.ModelNameOverride != *tt.backend.ModelNameOverride {
			t.Errorf("ResolveModel(%q, coding) = %+v, want binding %+v", tt.host, got, tt.backend)
		}
	}
}

func parseConfig(t *testing.T, compiled aidataplane.Config) *Config {
	t.Helper()
	data, err := aidataplane.Encode(compiled)
	if err != nil {
		t.Fatalf("Encode(test configuration) error = %v", err)
	}
	config, err := newConfig(data)
	if err != nil {
		t.Fatalf("newConfig(test configuration) error = %v", err)
	}

	return config
}

func configFixture(t *testing.T) aidataplane.Config {
	t.Helper()
	data, err := os.ReadFile("../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(configuration fixture) error = %v", err)
	}
	compiled, err := aidataplane.Decode(data)
	if err != nil {
		t.Fatalf("Decode(configuration fixture) error = %v", err)
	}

	return *compiled
}
