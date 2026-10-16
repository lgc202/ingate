package mcpprotocol

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	const valid = `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"weather","_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
	for _, tt := range []struct {
		name, body string
		change     func(http.Header)
		code       int
	}{
		{name: "valid", body: valid},
		{name: "literal sentinel prefix", body: strings.Replace(valid, "weather", "=?base64?weather", 1), change: func(h http.Header) { h.Set("Mcp-Name", "=?base64?weather") }},
		{name: "encoded name", body: valid, change: func(h http.Header) {
			h.Set("Mcp-Name", "=?base64?"+base64.StdEncoding.EncodeToString([]byte("weather"))+"?=")
		}},
		{name: "unicode name", body: strings.Replace(valid, "weather", "天气", 1), change: func(h http.Header) { h.Set("Mcp-Name", "=?base64?5aSp5rCU?=") }},
		{name: "literal sentinel", body: strings.Replace(valid, "weather", "=?base64?literal?=", 1), change: func(h http.Header) {
			h.Set("Mcp-Name", "=?base64?"+base64.StdEncoding.EncodeToString([]byte("=?base64?literal?="))+"?=")
		}},
		{name: "unclosed sentinel", body: valid, change: func(h http.Header) { h.Set("Mcp-Name", "=?base64?broken") }, code: CodeHeaderMismatch},
		{name: "malformed base64", body: valid, change: func(h http.Header) { h.Set("Mcp-Name", "=?base64?%%%?=") }, code: CodeHeaderMismatch},
		{name: "missing version", body: valid, change: func(h http.Header) { h.Del("MCP-Protocol-Version") }, code: CodeHeaderMismatch},
		{name: "unsupported version", body: valid, change: func(h http.Header) { h.Set("MCP-Protocol-Version", "2025-11-25") }, code: CodeUnsupportedProtocolVersion},
		{name: "duplicate version", body: valid, change: func(h http.Header) { h.Add("MCP-Protocol-Version", Version) }, code: CodeHeaderMismatch},
		{name: "joined version", body: valid, change: func(h http.Header) { h.Set("MCP-Protocol-Version", Version+", "+Version) }, code: CodeHeaderMismatch},
		{name: "missing method", body: valid, change: func(h http.Header) { h.Del("Mcp-Method") }, code: CodeHeaderMismatch},
		{name: "duplicate method", body: valid, change: func(h http.Header) { h.Add("Mcp-Method", "tools/call") }, code: CodeHeaderMismatch},
		{name: "mismatched method", body: valid, change: func(h http.Header) { h.Set("Mcp-Method", "tools/list") }, code: CodeHeaderMismatch},
		{name: "missing body name", body: strings.Replace(valid, `"name":"weather",`, "", 1), code: CodeHeaderMismatch},
		{name: "null body name", body: strings.Replace(valid, `"name":"weather"`, `"name":null`, 1), code: CodeHeaderMismatch},
		{name: "numeric body name", body: strings.Replace(valid, `"name":"weather"`, `"name":42`, 1), code: CodeHeaderMismatch},

		{name: "missing name", body: valid, change: func(h http.Header) { h.Del("Mcp-Name") }, code: CodeHeaderMismatch},
		{name: "duplicate name", body: valid, change: func(h http.Header) { h.Add("Mcp-Name", "weather") }, code: CodeHeaderMismatch},
		{name: "mismatched name", body: valid, change: func(h http.Header) { h.Set("Mcp-Name", "other") }, code: CodeHeaderMismatch},
		{name: "unsafe unencoded name", body: strings.Replace(valid, "weather", "天气", 1), change: func(h http.Header) { h.Set("Mcp-Name", "天气") }, code: CodeHeaderMismatch},
		{name: "mismatched body version", body: strings.Replace(valid, Version, "2025-11-25", 1), code: CodeHeaderMismatch},
		{name: "missing body metadata", body: `{"jsonrpc":"2.0","id":7,"method":"tools/list"}`, code: CodeHeaderMismatch},
		{name: "duplicate json method", body: strings.Replace(valid, `"method":"tools/call"`, `"method":"tools/list","method":"tools/call"`, 1), code: CodeInvalidRequest},
		{name: "null id", body: strings.Replace(valid, `"id":7`, `"id":null`, 1), code: CodeInvalidRequest},
		{name: "boolean id", body: strings.Replace(valid, `"id":7`, `"id":true`, 1), code: CodeInvalidRequest},
		{name: "fractional id", body: strings.Replace(valid, `"id":7`, `"id":1.5`, 1), code: CodeInvalidRequest},
		{name: "response envelope", body: `{"jsonrpc":"2.0","id":7,"result":{}}`, code: CodeInvalidRequest},
		{name: "mixed request response", body: strings.Replace(valid, `"id":7`, `"id":7,"result":{}`, 1), code: CodeInvalidRequest},
		{name: "batch", body: "[" + valid + "]", code: CodeInvalidRequest},
		{name: "invalid json", body: "{", code: CodeParseError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("MCP-Protocol-Version", Version)
			header.Set("Mcp-Method", "tools/call")
			header.Set("Mcp-Name", "weather")
			header.Set("Mcp-Param-Unrecognized", "=?base64?unknown?=")
			if tt.change != nil {
				tt.change(header)
			}

			body := []byte(tt.body)
			original := bytes.Clone(body)

			err := Validate(header, body)

			if tt.code == 0 {
				if err != nil {
					t.Fatalf("Validate() = %v", err)
				}
			} else {
				protocolErr, ok := errors.AsType[*Error](err)
				if !ok || protocolErr.Code != tt.code {
					t.Fatalf("Validate() = %v, want code %d", err, tt.code)
				}
			}

			if !bytes.Equal(body, original) {
				t.Fatal("Validate modified original bytes")
			}
		})
	}
}
