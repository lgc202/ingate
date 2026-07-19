package redis

import (
	"strings"
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

func TestTokenBucketCommandUsesRedisTimeAndExplicitCapacity(t *testing.T) {
	bucket, err := NewTokenBucket("limit-key", config.Quota{Requests: 100, WindowSeconds: 60, Burst: 20})
	if err != nil {
		t.Fatalf("NewTokenBucket() error = %v", err)
	}
	command, err := bucket.Command()
	if err != nil {
		t.Fatalf("TokenBucket.Command() error = %v", err)
	}
	value, err := Decode(command)
	if err != nil {
		t.Fatalf("Decode(TokenBucket.Command()) error = %v", err)
	}
	if len(value.Values) != 7 {
		t.Fatalf("TokenBucket.Command() argument count = %d, want 7", len(value.Values))
	}
	script := string(value.Values[1].Bytes)
	if !strings.Contains(script, `redis.call("TIME")`) {
		t.Errorf("TokenBucket.Command() script does not use Redis TIME")
	}
	gotArgs := []string{
		string(value.Values[4].Bytes),
		string(value.Values[5].Bytes),
		string(value.Values[6].Bytes),
	}
	wantArgs := []string{"20", "100", "60000"}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Errorf("TokenBucket.Command() argument %d = %q, want %q", i, gotArgs[i], wantArgs[i])
		}
	}
}

func TestTokenBucketParseResponseReturnsNonZeroResetForAllowedRequest(t *testing.T) {
	state, err := ParseBucketState([]byte("*5\r\n:1\r\n:1\r\n:20\r\n:19\r\n:3000\r\n"))
	if err != nil {
		t.Fatalf("ParseBucketState() error = %v", err)
	}
	if !state.Allowed || state.Limit != 20 || state.Remaining != 19 || state.ResetSeconds != 3 {
		t.Errorf("ParseBucketState() = %+v, want allowed state with limit 20, remaining 19 and reset 3", state)
	}
}

func TestNewTokenBucketDefaultsCapacityToRequests(t *testing.T) {
	bucket, err := NewTokenBucket("limit-key", config.Quota{Requests: 100, WindowSeconds: 60})
	if err != nil {
		t.Fatalf("NewTokenBucket(Burst=0) error = %v", err)
	}
	command, err := bucket.Command()
	if err != nil {
		t.Fatalf("TokenBucket.Command() error = %v", err)
	}
	value, err := Decode(command)
	if err != nil {
		t.Fatalf("Decode(TokenBucket.Command()) error = %v", err)
	}
	if got := string(value.Values[4].Bytes); got != "100" {
		t.Errorf("TokenBucket.Command() capacity = %q, want %q", got, "100")
	}
}
