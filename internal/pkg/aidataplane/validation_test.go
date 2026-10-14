package aidataplane

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"
)

func TestEncodeInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{
			name: "format",
			change: func(c *Config) {
				c.FormatVersion = "v2"
			},
		},
		{
			name: "nil listeners",
			change: func(c *Config) {
				c.Listeners = nil
			},
		},
		{
			name: "nil backends",
			change: func(c *Config) {
				c.Backends = nil
			},
		},
		{
			name: "nil network",
			change: func(c *Config) {
				c.NetworkBackends = nil
			},
		},
		{
			name: "empty network name",
			change: func(c *Config) {
				c.NetworkBackends[0].Name = ""
				c.Backends[0].NetworkBackendName = ""
			},
		},
		{
			name: "duplicate network",
			change: func(c *Config) {
				c.NetworkBackends = append(c.NetworkBackends, c.NetworkBackends[0])
			},
		},
		{
			name: "no endpoints",
			change: func(c *Config) {
				c.NetworkBackends[0].Endpoints = []Endpoint{}
			},
		},
		{
			name: "address",
			change: func(c *Config) {
				c.NetworkBackends[0].Endpoints[0].Address = "https://model.example"
			},
		},
		{
			name: "endpoint port",
			change: func(c *Config) {
				c.NetworkBackends[0].Endpoints[0].Port = 65536
			},
		},
		{
			name: "weight",
			change: func(c *Config) {
				c.NetworkBackends[0].Endpoints[0].Weight = 0
			},
		},
		{
			name: "duplicate endpoint",
			change: func(c *Config) {
				backend := &c.NetworkBackends[0]
				backend.Endpoints = append(backend.Endpoints, backend.Endpoints[0])
			},
		},
		{
			name: "empty backend name",
			change: func(c *Config) {
				c.Backends[0].Name = ""
				c.Listeners[0].Hosts[0].ModelRoutes[0].BackendName = ptr.To("")
			},
		},
		{
			name: "duplicate backend",
			change: func(c *Config) {
				c.Backends = append(c.Backends, c.Backends[0])
			},
		},
		{
			name: "schema",
			change: func(c *Config) {
				c.Backends[0].Schema.Name = "Anthropic"
			},
		},
		{
			name: "empty prefix",
			change: func(c *Config) {
				c.Backends[0].Schema.Prefix = ""
			},
		},
		{
			name: "nil override",
			change: func(c *Config) {
				c.Backends[0].ModelNameOverride = nil
			},
		},
		{
			name: "prefix authority",
			change: func(c *Config) {
				c.Backends[0].Schema.Prefix = "//model.example/v1"
			},
		},
		{
			name: "prefix escape",
			change: func(c *Config) {
				c.Backends[0].Schema.Prefix = "%zz"
			},
		},
		{
			name: "network reference",
			change: func(c *Config) {
				c.Backends[0].NetworkBackendName = "missing"
			},
		},
		{
			name: "listener port",
			change: func(c *Config) {
				c.Listeners[0].Port = 0
			},
		},
		{
			name: "duplicate port",
			change: func(c *Config) {
				c.Listeners = append(c.Listeners, c.Listeners[0])
			},
		},
		{
			name: "nil hosts",
			change: func(c *Config) {
				c.Listeners[0].Hosts = nil
			},
		},
		{
			name: "empty host name",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].Name = ""
			},
		},
		{
			name: "duplicate host",
			change: func(c *Config) {
				listener := &c.Listeners[0]
				listener.Hosts = append(listener.Hosts, listener.Hosts[0])
			},
		},
		{
			name: "wildcard host",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].Hostname = ptr.To("*.example.com")
			},
		},
		{
			name: "nil hostname",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].Hostname = nil
			},
		},
		{
			name: "nil models",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].ModelRoutes = nil
			},
		},
		{
			name: "model name",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].ModelRoutes[0].Name = ""
			},
		},
		{
			name: "duplicate model",
			change: func(c *Config) {
				host := &c.Listeners[0].Hosts[0]
				host.ModelRoutes = append(host.ModelRoutes, host.ModelRoutes[0])
			},
		},
		{
			name: "nil headers",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].ModelRoutes[0].Headers = nil
			},
		},
		{
			name: "two headers",
			change: func(c *Config) {
				route := &c.Listeners[0].Hosts[0].ModelRoutes[0]
				route.Headers = append(route.Headers, route.Headers[0])
			},
		},
		{
			name: "header name",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].ModelRoutes[0].Headers[0].Name = "x-other"
			},
		},
		{
			name: "header type",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].ModelRoutes[0].Headers[0].Type = "RegularExpression"
			},
		},
		{
			name: "empty model",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].ModelRoutes[0].Headers[0].Value = ""
			},
		},
		{
			name: "model reference",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].ModelRoutes[0].BackendName = ptr.To("missing")
			},
		},
		{
			name: "nil model reference",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].ModelRoutes[0].BackendName = nil
			},
		},
		{
			name: "nil mcp",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].MCPRoutes = nil
			},
		},
		{
			name: "mcp name",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].MCPRoutes[0].Name = ""
			},
		},
		{
			name: "duplicate mcp",
			change: func(c *Config) {
				host := &c.Listeners[0].Hosts[0]
				host.MCPRoutes = append(host.MCPRoutes, host.MCPRoutes[0])
			},
		},
		{
			name: "mcp relative path",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].MCPRoutes[0].Path = "mcp"
			},
		},
		{
			name: "mcp query",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].MCPRoutes[0].Path = "/mcp?token=x"
			},
		},
		{
			name: "mcp backend authority",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].MCPRoutes[0].BackendPath = "//other/stream"
			},
		},
		{
			name: "mcp long path",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].MCPRoutes[0].BackendPath = "/" + strings.Repeat("a", 1024)
			},
		},
		{
			name: "mcp reference",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].MCPRoutes[0].BackendName = ptr.To("missing")
			},
		},
		{
			name: "nil mcp reference",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].MCPRoutes[0].BackendName = nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := loadFixture(t)
			tt.change(&c)

			data, err := Encode(c)
			if err == nil || data != nil {
				t.Errorf("Encode(%s) data/error = %s/%v, want nil data and validation error", tt.name, data, err)
			}
		})
	}
}

func TestEncodeUnavailableBackends(t *testing.T) {
	c, _ := loadFixture(t)
	host := &c.Listeners[0].Hosts[0]
	host.Hostname = ptr.To("")
	host.ModelRoutes[0].Headers = []HeaderMatch{}
	host.ModelRoutes[0].BackendName = ptr.To("")
	host.MCPRoutes[0].BackendName = ptr.To("")
	c.Backends = []Backend{}
	c.NetworkBackends = []NetworkBackend{}

	data, err := Encode(c)
	if err != nil {
		t.Fatalf("Encode(unavailable targets and catch-all) error = %v", err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal(unavailable targets and catch-all) error = %v", err)
	}

	c.Revision = decoded.Revision
	if diff := cmp.Diff(c, decoded); diff != "" {
		t.Errorf("Encode(unavailable targets and catch-all) changed retained routes (-want +got):\n%s", diff)
	}
}

func TestEncodeIPAddresses(t *testing.T) {
	for _, address := range []string{"192.0.2.1", "2001:db8::1"} {
		t.Run(address, func(t *testing.T) {
			c, _ := loadFixture(t)
			c.NetworkBackends[0].Endpoints[0].Address = address

			if _, err := Encode(c); err != nil {
				t.Errorf("Encode(address %q) error = %v, want nil", address, err)
			}
		})
	}
}
