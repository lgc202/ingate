package openai

import (
	"encoding/json"
	"testing"
)

func TestDefaultErrorAndEncodeError(t *testing.T) {
	detail := DefaultError(429, "too many requests")
	if detail.Type != "rate_limit_error" || detail.Code != "429" {
		t.Errorf("DefaultError(429, ...) = %#v, want rate_limit_error with code 429", detail)
	}

	body := EncodeError(detail)
	var envelope ErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(EncodeError(...)) returned error: %v", err)
	}
	if envelope.Error != detail {
		t.Errorf("EncodeError(%#v) decoded as %#v", detail, envelope.Error)
	}
	if string(body) != `{"error":{"message":"too many requests","type":"rate_limit_error","param":null,"code":"429"}}` {
		t.Errorf("EncodeError(%#v) = %s, want param:null", detail, body)
	}
}
