package endpointspec_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/endpointspec"
)

func TestParseResponses(t *testing.T) {
	for _, body := range []string{
		`{"model":"coding","input":"hello"}`,
		`{"model":"coding","input":[{"role":"user","content":"hello"}],"stream":true}`,
		`{"model":"cod\u0069ng","previous_response_id":"resp_1"}`,
		`{"\u006dodel":"coding","background":false}`,
		`{"model":"coding","background":null,"conversation":"conv_1"}`,
	} {
		request, err := endpointspec.ParseResponses([]byte(body))
		if err != nil {
			t.Fatalf("ParseResponses(%q) error = %v, want nil", body, err)
		}

		if request.Model != "coding" || request.Background {
			t.Errorf("ParseResponses(%q) = %+v, want coding and non-background request", body, request)
		}
	}
}

func TestParseResponsesRejectsInvalidRequests(t *testing.T) {
	for _, body := range []string{
		"", `null`, `[]`, `"coding"`, `{"model":`, `{"model":"coding"} {}`,
		`{}`, `{"Model":"coding"}`, `{"model":null}`, `{"model":""}`, `{"model":23}`,
		`{"model":true}`, `{"model":{}}`, `{"model":["coding"]}`,
		`{"model":"coding","model":"other"}`, `{"model":"coding","\u006dodel":"other"}`,
		`{"model":"coding","background":false,"background":true}`,
		`{"model":"coding","background":true}`, `{"model":"coding","background":"false"}`,
		`{"model":"coding","background":1}`, `{"model":"coding","extra":1,"extra":2}`,
		"{\"model\":\"\xff\"}",
	} {
		request, err := endpointspec.ParseResponses([]byte(body))

		if request != nil || err == nil {
			t.Errorf("ParseResponses(%q) = (%+v, %v), want nil and error", body, request, err)
		}
	}
}

func TestParseResponsesPreservesErrorCause(t *testing.T) {
	_, err := endpointspec.ParseResponses([]byte(`{"model":"coding","background":"true"}`))

	if _, ok := errors.AsType[*json.UnmarshalTypeError](err); !ok {
		t.Errorf("ParseResponses(string background) error = %v, want wrapped JSON type error", err)
	}
}

func TestParseResponsesOwnsModel(t *testing.T) {
	body := []byte(`{"model":"cod\u0069ng","input":"hello"}`)
	request, err := endpointspec.ParseResponses(body)
	if err != nil {
		t.Fatalf("ParseResponses(valid body) error = %v", err)
	}

	for i := range body {
		body[i] = 'x'
	}

	if request.Model != "coding" {
		t.Errorf("ParseResponses() model after source mutation = %q, want coding", request.Model)
	}
}
