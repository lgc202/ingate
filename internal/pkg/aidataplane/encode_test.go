package aidataplane

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"
)

// TestEncodeGolden 固定字段顺序与文档示例的内容哈希，不只比较 JSON 语义。
func TestEncodeGolden(t *testing.T) {
	c, golden := loadFixture(t)
	c.Revision = "stale"
	before, _ := loadFixture(t)
	before.Revision = "stale"

	data, err := Encode(c)
	if err != nil {
		t.Fatalf("Encode(golden) error = %v", err)
	}

	if !bytes.Equal(data, golden) {
		t.Errorf("Encode(golden) = %s, want %s", data, golden)
	}

	if diff := cmp.Diff(before, c); diff != "" {
		t.Errorf("Encode(golden) changed input (-want +got):\n%s", diff)
	}

	repeated, err := Encode(c)
	if err != nil {
		t.Fatalf("Encode(golden repeated) error = %v", err)
	}

	if !bytes.Equal(repeated, data) {
		t.Errorf("Encode(golden repeated) = %s, want %s", repeated, data)
	}
}

func TestEncodeEmpty(t *testing.T) {
	c := Config{
		FormatVersion:   FormatVersionV1,
		Listeners:       []Listener{},
		Backends:        []Backend{},
		NetworkBackends: []NetworkBackend{},
	}

	data, err := Encode(c)
	if err != nil {
		t.Fatalf("Encode(empty) error = %v", err)
	}

	const want = `{"formatVersion":"v1","revision":"846511313711e1ee10dc7d71b68284a2900062756b66f5d3ac428294e9017a0d","listeners":[],"backends":[],"networkBackends":[]}`
	if string(data) != want {
		t.Errorf("Encode(empty) = %s, want %s", data, want)
	}
}

func TestEncodePreservesRouteOrder(t *testing.T) {
	c, _ := loadFixture(t)
	host := &c.Listeners[0].Hosts[0]
	catchAll := ModelRoute{
		Name:        "team-a/z/rule/0/match/0",
		Headers:     []HeaderMatch{},
		BackendName: ptr.To(""),
	}
	host.ModelRoutes = append([]ModelRoute{catchAll}, host.ModelRoutes...)

	data, err := Encode(c)
	if err != nil {
		t.Fatalf("Encode(catch-all first) error = %v", err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal(encoded order) error = %v", err)
	}

	if diff := cmp.Diff(host.ModelRoutes, decoded.Listeners[0].Hosts[0].ModelRoutes); diff != "" {
		t.Errorf("Encode(catch-all first) reordered routes (-want +got):\n%s", diff)
	}
}

func TestEncodeRevisionChanges(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{
			name: "model",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].ModelRoutes[0].Headers[0].Value = "other"
			},
		},
		{
			name: "override",
			change: func(c *Config) {
				c.Backends[0].ModelNameOverride = ptr.To("other")
			},
		},
		{
			name: "prefix",
			change: func(c *Config) {
				c.Backends[0].Schema.Prefix = "v2/"
			},
		},
		{
			name: "path",
			change: func(c *Config) {
				c.Listeners[0].Hosts[0].MCPRoutes[0].BackendPath = "/stream/"
			},
		},
		{
			name: "endpoint",
			change: func(c *Config) {
				c.NetworkBackends[0].Endpoints[0].Weight = 2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, golden := loadFixture(t)
			before := c.Revision
			tt.change(&c)

			data, err := Encode(c)
			if err != nil {
				t.Fatalf("Encode(%s) error = %v", tt.name, err)
			}

			if bytes.Equal(data, golden) {
				t.Errorf("Encode(%s) = unchanged golden, want changed output", tt.name)
			}

			var decoded Config
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal(Encode(%s)) error = %v", tt.name, err)
			}

			if decoded.Revision == before {
				t.Errorf("Encode(%s) revision = %q, want different from %q", tt.name, decoded.Revision, before)
			}
		})
	}
}

func TestEncodePathError(t *testing.T) {
	c, _ := loadFixture(t)
	c.Backends[0].Schema.Prefix = "%zz"

	_, err := Encode(c)

	if _, ok := errors.AsType[*url.Error](err); !ok {
		t.Errorf("Encode(prefix %%zz) error = %v, want wrapped *url.Error", err)
	}
}

// loadFixture 保留 JSON 字段顺序和字面量，仅移除展示空白以核对编码契约。
func loadFixture(t *testing.T) (Config, []byte) {
	t.Helper()

	data, err := os.ReadFile("testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(testdata/v1.json) error = %v", err)
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("Unmarshal(testdata/v1.json) error = %v", err)
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		t.Fatalf("Compact(testdata/v1.json) error = %v", err)
	}

	return c, compact.Bytes()
}
