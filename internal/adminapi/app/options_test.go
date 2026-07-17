package app

import (
	"testing"
	"time"
)

func TestNewOptionsControllerStatusDefaults(t *testing.T) {
	options := NewOptions()
	if options.ControllerStatusURL != "http://127.0.0.1:18080" {
		t.Fatalf("ControllerStatusURL = %q, want %q", options.ControllerStatusURL, "http://127.0.0.1:18080")
	}
	if options.ControllerStatusTimeout != 500*time.Millisecond {
		t.Fatalf("ControllerStatusTimeout = %s, want %s", options.ControllerStatusTimeout, 500*time.Millisecond)
	}
}
