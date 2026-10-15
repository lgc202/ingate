package aidataplane

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"
	sigsjson "sigs.k8s.io/json"
)

func TestDecodeGolden(t *testing.T) {
	want, golden := loadFixture(t)
	got, err := Decode(golden)
	if err != nil || got == nil {
		t.Fatalf("Decode(golden) config/error = %+v/%v, want non-nil config and nil error", got, err)
	}

	if diff := cmp.Diff(&want, got); diff != "" {
		t.Errorf("Decode(golden) mismatch (-want +got):\n%s", diff)
	}

	encoded, err := Encode(*got)
	if err != nil {
		t.Fatalf("Encode(Decode(golden)) error = %v", err)
	}

	if !bytes.Equal(encoded, golden) {
		t.Errorf("Encode(Decode(golden)) = %s, want %s", encoded, golden)
	}
}

func TestDecodeJSONFormatting(t *testing.T) {
	want, golden := loadFixture(t)
	var document map[string]any
	if err := json.Unmarshal(golden, &document); err != nil {
		t.Fatalf("Unmarshal(golden document) error = %v", err)
	}
	formatted, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(golden document) error = %v", err)
	}

	for _, data := range [][]byte{
		formatted,
		bytes.Replace(golden, []byte(`"value":"coding"`), []byte(`"value":"\u0063oding"`), 1),
	} {
		got, err := Decode(data)
		if err != nil {
			t.Fatalf("Decode(reformatted JSON) error = %v", err)
		}

		if diff := cmp.Diff(&want, got); diff != "" {
			t.Errorf("Decode(reformatted JSON) mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestDecodeEmpty(t *testing.T) {
	want := Config{
		FormatVersion:   FormatVersionV1,
		Listeners:       []Listener{},
		Backends:        []Backend{},
		NetworkBackends: []NetworkBackend{},
	}
	data, err := Encode(want)
	if err != nil {
		t.Fatalf("Encode(empty) error = %v", err)
	}

	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode(empty) error = %v", err)
	}

	want.Revision = "846511313711e1ee10dc7d71b68284a2900062756b66f5d3ac428294e9017a0d"
	if diff := cmp.Diff(&want, got); diff != "" {
		t.Errorf("Decode(empty) mismatch (-want +got):\n%s", diff)
	}
}

func TestDecodeEmptyRequiredStrings(t *testing.T) {
	want, golden := loadFixture(t)
	document, objects := fixtureObjects(t, golden)
	objects["host"]["hostname"] = ""
	objects["model"]["backendName"] = ""
	objects["mcp"]["backendName"] = ""
	objects["backend"]["modelNameOverride"] = ""
	data := documentWithRevision(t, document)

	got, err := Decode(data)
	if err != nil || got == nil {
		t.Fatalf("Decode(empty required strings) config/error = %+v/%v, want non-nil config and nil error", got, err)
	}

	want.Listeners[0].Hosts[0].Hostname = ptr.To("")
	want.Listeners[0].Hosts[0].ModelRoutes[0].BackendName = ptr.To("")
	want.Listeners[0].Hosts[0].MCPRoutes[0].BackendName = ptr.To("")
	want.Backends[0].ModelNameOverride = ptr.To("")
	want.Revision = got.Revision
	if diff := cmp.Diff(&want, got); diff != "" {
		t.Errorf("Decode(empty required strings) mismatch (-want +got):\n%s", diff)
	}

	encoded, err := Encode(*got)
	if err != nil {
		t.Fatalf("Encode(empty required strings) error = %v", err)
	}
	roundTrip, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode(Encode(empty required strings)) error = %v", err)
	}

	if diff := cmp.Diff(&want, roundTrip); diff != "" {
		t.Errorf("Decode(Encode(empty required strings)) mismatch (-want +got):\n%s", diff)
	}
}

// TestDecodeRequiredFields 对示例中的每种对象逐字段删除或置 null。
// 重新计算剩余强类型内容的正确哈希，避免只靠哈希不匹配误判字段校验通过。
func TestDecodeRequiredFields(t *testing.T) {
	_, golden := loadFixture(t)
	_, objects := fixtureObjects(t, golden)

	for location, object := range objects {
		for field := range object {
			if location == "config" && field == "revision" {
				continue // revision 的缺失/null 单独验证，不能在准备输入时补回。
			}
			for _, operation := range []string{"missing", "null"} {
				t.Run(location+"."+field+"."+operation, func(t *testing.T) {
					document, targets := fixtureObjects(t, golden)
					if operation == "missing" {
						delete(targets[location], field)
					} else {
						targets[location][field] = nil
					}
					data := documentWithRevision(t, document)

					got, err := Decode(data)
					if err == nil || got != nil {
						t.Errorf("Decode(%s.%s %s) config/error = %+v/%v, want nil config and error", location, field, operation, got, err)
					}
				})
			}
		}
	}
}

func TestDecodeStrictJSON(t *testing.T) {
	_, golden := loadFixture(t)
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty"},
		{name: "root null", data: []byte("null")},
		{name: "root array", data: []byte("[]")},
		{name: "syntax", data: []byte("{")},
		{name: "trailing value", data: append(bytes.Clone(golden), []byte(" {}")...)},
		{
			name: "unknown root",
			data: bytes.Replace(golden, []byte(`"formatVersion"`), []byte(`"extra":true,"formatVersion"`), 1),
		},
		{
			name: "unknown nested",
			data: bytes.Replace(golden, []byte(`"schema":{`), []byte(`"schema":{"extra":true,`), 1),
		},
		{
			name: "wrong case",
			data: bytes.Replace(golden, []byte(`"formatVersion"`), []byte(`"FormatVersion"`), 1),
		},
		{
			name: "duplicate root",
			data: bytes.Replace(golden, []byte(`"formatVersion":"v1"`), []byte(`"formatVersion":"v1","formatVersion":"v1"`), 1),
		},
		{
			name: "escaped duplicate",
			data: bytes.Replace(golden, []byte(`"formatVersion":"v1"`), []byte(`"formatVersion":"v1","\u0066ormatVersion":"v1"`), 1),
		},
		{
			name: "duplicate nested",
			data: bytes.Replace(golden, []byte(`"name":"x-ai-eg-model"`), []byte(`"name":"x-ai-eg-model","name":"x-ai-eg-model"`), 1),
		},
		{
			name: "wrong type",
			data: bytes.Replace(golden, []byte(`"port":8080`), []byte(`"port":"8080"`), 1),
		},
		{
			name: "fractional port",
			data: bytes.Replace(golden, []byte(`"port":8080`), []byte(`"port":8080.5`), 1),
		},
		{
			name: "overflow port",
			data: bytes.Replace(golden, []byte(`"port":8080`), []byte(`"port":2147483648`), 1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode(tt.data)
			if err == nil || got != nil {
				t.Errorf("Decode(%s) config/error = %+v/%v, want nil config and error", tt.name, got, err)
			}
		})
	}
}

func TestDecodeRevision(t *testing.T) {
	_, golden := loadFixture(t)
	tests := []struct {
		name     string
		revision any
	}{
		{name: "null"},
		{name: "empty", revision: ""},
		{name: "short", revision: "ab"},
		{name: "uppercase", revision: strings.Repeat("A", 64)},
		{name: "not hex", revision: strings.Repeat("g", 64)},
		{name: "mismatch", revision: strings.Repeat("0", 64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, _ := fixtureObjects(t, golden)
			document["revision"] = tt.revision
			data, err := json.Marshal(document)
			if err != nil {
				t.Fatalf("Marshal(%s revision) error = %v", tt.name, err)
			}

			got, err := Decode(data)
			if err == nil || got != nil {
				t.Errorf("Decode(%s revision) config/error = %+v/%v, want nil config and error", tt.name, got, err)
			}
		})
	}

	document, _ := fixtureObjects(t, golden)
	delete(document, "revision")
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("Marshal(missing revision) error = %v", err)
	}

	if got, err := Decode(data); err == nil || got != nil {
		t.Errorf("Decode(missing revision) config/error = %+v/%v, want nil config and error", got, err)
	}

	data = bytes.Replace(golden, []byte(`"value":"coding"`), []byte(`"value":"other"`), 1)

	if got, err := Decode(data); err == nil || got != nil {
		t.Errorf("Decode(changed content) config/error = %+v/%v, want nil config and error", got, err)
	}
}

func TestDecodeSemanticConfig(t *testing.T) {
	_, golden := loadFixture(t)
	tests := []struct {
		name     string
		location string
		field    string
		value    any
	}{
		{name: "format", location: "config", field: "formatVersion", value: "v2"},
		{name: "schema", location: "schema", field: "name", value: "Anthropic"},
		{name: "model reference", location: "model", field: "backendName", value: "missing"},
		{name: "network reference", location: "backend", field: "networkBackendName", value: "missing"},
		{name: "mcp reference", location: "mcp", field: "backendName", value: "missing"},
		{name: "port range", location: "endpoint", field: "port", value: 65536},
		{name: "weight", location: "endpoint", field: "weight", value: 0},
		{name: "match type", location: "header", field: "type", value: "RegularExpression"},
		{name: "prefix authority", location: "schema", field: "prefix", value: "https://other.example/v1"},
		{name: "mcp path", location: "mcp", field: "path", value: "mcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, objects := fixtureObjects(t, golden)
			objects[tt.location][tt.field] = tt.value
			data := documentWithRevision(t, document)

			got, err := Decode(data)
			if err == nil || got != nil {
				t.Errorf("Decode(%s with valid hash) config/error = %+v/%v, want nil config and error", tt.name, got, err)
			}
		})
	}
}

func TestDecodeFieldError(t *testing.T) {
	_, golden := loadFixture(t)
	data := bytes.Replace(golden, []byte(`"formatVersion"`), []byte(`"extra":true,"formatVersion"`), 1)

	_, err := Decode(data)

	if fieldError, ok := errors.AsType[sigsjson.FieldError](err); !ok || fieldError.FieldPath() != "extra" {
		t.Errorf("Decode(unknown extra) error = %v, want wrapped FieldError for extra", err)
	}
}

func TestDecodeInvalidUTF8(t *testing.T) {
	c, _ := loadFixture(t)
	c.Listeners[0].Hosts[0].ModelRoutes[0].Headers[0].Value = "\ufffd"
	data, err := Encode(c)
	if err != nil {
		t.Fatalf("Encode(replacement rune) error = %v", err)
	}
	data = bytes.Replace(data, []byte("\ufffd"), []byte{0xff}, 1)

	if got, err := Decode(data); err == nil || got != nil {
		t.Errorf("Decode(invalid UTF-8) config/error = %+v/%v, want nil config and error", got, err)
	}
}

func TestDecodeOwnership(t *testing.T) {
	want, golden := loadFixture(t)
	before := bytes.Clone(golden)

	first, err := Decode(golden)
	if err != nil || first == nil {
		t.Fatalf("Decode(first) config/error = %+v/%v, want non-nil config and nil error", first, err)
	}
	second, err := Decode(golden)
	if err != nil || second == nil {
		t.Fatalf("Decode(second) config/error = %+v/%v, want non-nil config and nil error", second, err)
	}

	if !bytes.Equal(before, golden) {
		t.Error("Decode(golden) changed input bytes, want unchanged input")
	}

	first.Listeners[0].Hosts[0].ModelRoutes[0].Headers[0].Value = "other"
	*first.Listeners[0].Hosts[0].Hostname = "other.example.com"
	*first.Listeners[0].Hosts[0].ModelRoutes[0].BackendName = "other"
	*first.Listeners[0].Hosts[0].MCPRoutes[0].BackendName = "other"
	*first.Backends[0].ModelNameOverride = "other"
	first.NetworkBackends[0].Endpoints[0].Weight = 99
	clear(golden)

	if diff := cmp.Diff(&want, second); diff != "" {
		t.Errorf("Decode(second) aliases input or another result (-want +got):\n%s", diff)
	}
}

// fixtureObjects 仅展开固定 golden 示例，供字段缺失与 null 的反例使用。
func fixtureObjects(t *testing.T, data []byte) (map[string]any, map[string]map[string]any) {
	t.Helper()

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("Unmarshal(fixture document) error = %v", err)
	}

	listener := firstFixtureObject(t, document, "listeners")
	host := firstFixtureObject(t, listener, "hosts")
	model := firstFixtureObject(t, host, "modelRoutes")
	mcp := firstFixtureObject(t, host, "mcpRoutes")
	backend := firstFixtureObject(t, document, "backends")
	network := firstFixtureObject(t, document, "networkBackends")
	objects := map[string]map[string]any{
		"config":   document,
		"listener": listener,
		"host":     host,
		"model":    model,
		"header":   firstFixtureObject(t, model, "headers"),
		"mcp":      mcp,
		"backend":  backend,
		"schema":   fixtureObject(t, backend["schema"], "schema"),
		"network":  network,
		"endpoint": firstFixtureObject(t, network, "endpoints"),
	}
	return document, objects
}

// documentWithRevision 为单项非法内容准备正确哈希，不调用会拒绝非法值的 Encode。
func documentWithRevision(t *testing.T, document map[string]any) []byte {
	t.Helper()

	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("Marshal(modified document) error = %v", err)
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("Unmarshal(modified document) error = %v", err)
	}

	revision, err := contentRevision(c)
	if err != nil {
		t.Fatalf("contentRevision(modified document) error = %v", err)
	}
	document["revision"] = revision

	data, err = json.Marshal(document)
	if err != nil {
		t.Fatalf("Marshal(modified document revision) error = %v", err)
	}
	return data
}

func firstFixtureObject(t *testing.T, parent map[string]any, field string) map[string]any {
	t.Helper()

	values, ok := parent[field].([]any)
	if !ok || len(values) == 0 {
		t.Fatalf("fixture %s = %#v, want non-empty array", field, parent[field])
	}
	return fixtureObject(t, values[0], field+"[0]")
}

func fixtureObject(t *testing.T, value any, field string) map[string]any {
	t.Helper()

	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("fixture %s = %#v, want object", field, value)
	}
	return object
}
