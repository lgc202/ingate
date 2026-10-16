package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/lgc202/ingate/internal/aiproxy/errno"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

type entryBody struct {
	*strings.Reader
	read bool
}

func TestModelEntryRejectsInvalidHeadersBeforeBodyRead(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ports []string
		hosts []string
	}{
		{name: "missing port", hosts: []string{"gateway/team-a/public/listener/api"}},
		{name: "missing host", ports: []string{"8080"}},
		{name: "duplicate port", ports: []string{"8080", "9090"}, hosts: []string{"host"}},
		{name: "duplicate host", ports: []string{"8080"}, hosts: []string{"host-a", "host-b"}},
		{name: "combined port", ports: []string{"8080,9090"}, hosts: []string{"host"}},
		{name: "combined host", ports: []string{"8080"}, hosts: []string{"host-a,host-b"}},
		{name: "empty host", ports: []string{"8080"}, hosts: []string{""}},
		{name: "zero port", ports: []string{"0"}, hosts: []string{"host"}},
		{name: "overflow port", ports: []string{"65536"}, hosts: []string{"host"}},
		{name: "invalid port", ports: []string{"http"}, hosts: []string{"host"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &entryBody{Reader: strings.NewReader(`{"model":"coding"}`)}
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
			for _, port := range tt.ports {
				request.Header.Add(aidataplane.ListenerPortHeader, port)
			}

			for _, host := range tt.hosts {
				request.Header.Add(aidataplane.HostNameHeader, host)
			}

			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
			response := httptest.NewRecorder()

			trustedEntry(errno.ErrInvalidModelEntry, encodeModelError)(next).ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest || called || body.read {
				t.Errorf("modelEntry(%s) status/next/body = %d/%t/%t, want 400/false/false", tt.name, response.Code, called, body.read)
			}

			if response.Header().Get("Content-Type") != "application/json" {
				t.Errorf("modelEntry(%s) Content-Type = %q, want application/json", tt.name, response.Header().Get("Content-Type"))
			}

			want := map[string]any{"error": map[string]any{
				"message": "invalid model entry", "type": "invalid_request_error", "param": nil, "code": "INVALID_MODEL_ENTRY",
			}}
			if diff := cmp.Diff(want, decodeModelError(t, response)); diff != "" {
				t.Errorf("modelEntry(%s) response mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func (b *entryBody) Read(p []byte) (int, error) {
	b.read = true

	return b.Reader.Read(p)
}
