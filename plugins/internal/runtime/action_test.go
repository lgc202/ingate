package runtime

import "testing"

func TestRespondActionCarriesDirectResponse(t *testing.T) {
	action := Respond(429, map[string]string{"X-RateLimit-Remaining": "0"}, "Too many requests")

	if action.Kind != ActionRespond {
		t.Fatalf("Kind = %q, want %q", action.Kind, ActionRespond)
	}
	if action.StatusCode != 429 {
		t.Fatalf("StatusCode = %d, want 429", action.StatusCode)
	}
	if action.Headers["X-RateLimit-Remaining"] != "0" {
		t.Fatalf("quota header = %q, want 0", action.Headers["X-RateLimit-Remaining"])
	}
	if action.Body != "Too many requests" {
		t.Fatalf("Body = %q, want Too many requests", action.Body)
	}
}
