package llm

import (
	"encoding/json"
	"testing"
)

func TestDefaultAPIErrorAndEncodeError(t *testing.T) {
	detail := DefaultAPIError(429, "too many requests")
	if detail.Type != "rate_limit_error" || detail.Code != "429" {
		t.Errorf("DefaultAPIError(429, ...) = %#v, want rate_limit_error with code 429", detail)
	}

	body := EncodeError(detail)
	var envelope ErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(EncodeError(...)) returned error: %v", err)
	}
	if envelope.Error != detail {
		t.Errorf("EncodeError(%#v) decoded as %#v", detail, envelope.Error)
	}
}
