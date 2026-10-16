package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

			modelEntry(next).ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest || called || body.read {
				t.Errorf("modelEntry(%s) status/next/body = %d/%t/%t, want 400/false/false", tt.name, response.Code, called, body.read)
			}

			var result struct {
				Code    int    `json:"code"`
				Reason  string `json:"reason"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode modelEntry(%s) response: %v", tt.name, err)
			}

			if result.Code != http.StatusBadRequest || result.Reason != "INVALID_MODEL_ENTRY" || result.Message != "invalid model entry" {
				t.Errorf("modelEntry(%s) response = %+v, want 400/INVALID_MODEL_ENTRY/invalid model entry", tt.name, result)
			}
		})
	}
}

func (b *entryBody) Read(p []byte) (int, error) {
	b.read = true

	return b.Reader.Read(p)
}
