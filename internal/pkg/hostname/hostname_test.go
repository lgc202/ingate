package hostname

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
		ok    bool
	}{
		{name: "empty catch all", want: "*", ok: true},
		{name: "explicit catch all", value: "*", want: "*", ok: true},
		{name: "exact", value: "API.Example.COM", want: "api.example.com", ok: true},
		{name: "wildcard", value: "*.Example.COM", want: "*.example.com", ok: true},
		{name: "surrounding whitespace", value: " api.example.com ", ok: false},
		{name: "invalid wildcard", value: "api.*.example.com", want: "api.*.example.com", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Normalize(tt.value)
			if got != tt.want || ok != tt.ok {
				t.Errorf("Normalize(%q) = (%q, %t), want (%q, %t)", tt.value, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestOverlaps(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		second string
		want   bool
	}{
		{name: "catch all and exact", first: "*", second: "api.example.com", want: true},
		{name: "same exact", first: "api.example.com", second: "api.example.com", want: true},
		{name: "different exact", first: "api.example.com", second: "mcp.example.com", want: false},
		{name: "wildcard and exact", first: "*.example.com", second: "api.example.com", want: true},
		{name: "wildcard and nested exact", first: "*.example.com", second: "deep.api.example.com", want: true},
		{name: "wildcard does not include apex", first: "*.example.com", second: "example.com", want: false},
		{name: "nested wildcards", first: "*.example.com", second: "*.api.example.com", want: true},
		{name: "separate wildcards", first: "*.api.example.com", second: "*.mcp.example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Overlaps(tt.first, tt.second); got != tt.want {
				t.Errorf("Overlaps(%q, %q) = %t, want %t", tt.first, tt.second, got, tt.want)
			}
		})
	}
}
