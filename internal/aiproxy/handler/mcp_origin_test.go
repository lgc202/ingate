package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSameOriginMCP(t *testing.T) {
	for _, tt := range []struct {
		name, host string
		origins    []string
		want       bool
	}{
		{name: "absent", host: "example.com", want: true},
		{name: "same", host: "example.com", origins: []string{"http://example.com"}, want: true},
		{name: "default port", host: "example.com:80", origins: []string{"http://example.com"}, want: true},
		{name: "default port origin", host: "example.com", origins: []string{"http://example.com:80"}, want: true},
		{name: "case", host: "Example.COM", origins: []string{"http://example.com"}, want: true},
		{name: "ipv6", host: "[::1]:80", origins: []string{"http://[::1]"}, want: true},
		{name: "ipv6 canonical", host: "[0:0:0:0:0:0:0:1]", origins: []string{"http://[::1]:80"}, want: true},
		{name: "cross host", host: "example.com", origins: []string{"http://evil.example"}},
		{name: "cross scheme", host: "example.com", origins: []string{"https://example.com"}},
		{name: "cross port", host: "example.com:8080", origins: []string{"http://example.com"}},
		{name: "null", host: "example.com", origins: []string{"null"}},
		{name: "empty", host: "example.com", origins: []string{""}},
		{name: "duplicate", host: "example.com", origins: []string{"http://example.com", "http://example.com"}},
		{name: "multiple", host: "example.com", origins: []string{"http://example.com http://evil.example"}},
		{name: "userinfo", host: "example.com", origins: []string{"http://user@example.com"}},
		{name: "path", host: "example.com", origins: []string{"http://example.com/"}},
		{name: "empty fragment", host: "example.com", origins: []string{"http://example.com#"}},
		{name: "empty query", host: "example.com", origins: []string{"http://example.com?"}},
		{name: "invalid host", host: "example.com:", origins: []string{"http://example.com:"}},
		{name: "invalid port", host: "example.com:99999", origins: []string{"http://example.com:99999"}},
		{name: "unbracketed ipv6", host: "::1", origins: []string{"http://[::1]"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://example.com/mcp", nil)
			request.Host = tt.host
			if tt.origins != nil {
				request.Header["Origin"] = tt.origins
			}

			request.Header.Set("Forwarded", "proto=https;host=evil.example")
			request.Header.Set("Sec-Fetch-Site", "same-origin")

			got := sameOriginMCP(request)

			if got != tt.want {
				t.Errorf("sameOriginMCP(%q, %v)=%t, want %t", tt.host, tt.origins, got, tt.want)
			}
		})
	}
}
