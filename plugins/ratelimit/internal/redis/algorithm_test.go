package redis

import (
	"strings"
	"testing"
	"time"
)

func TestBuildCommandUsesRedisTimeAndExplicitCapacity(t *testing.T) {
	command, err := BuildCommand(Request{
		Key:      "limit-key",
		Requests: 100,
		Window:   time.Minute,
		Capacity: 20,
	})
	if err != nil {
		t.Fatalf("BuildCommand() error = %v", err)
	}
	value, err := Decode(command)
	if err != nil {
		t.Fatalf("Decode(BuildCommand()) error = %v", err)
	}
	if len(value.Values) != 7 {
		t.Fatalf("BuildCommand() argument count = %d, want 7", len(value.Values))
	}
	script := string(value.Values[1].Bytes)
	if !strings.Contains(script, `redis.call("TIME")`) {
		t.Errorf("BuildCommand() script does not use Redis TIME")
	}
	gotArgs := []string{
		string(value.Values[4].Bytes),
		string(value.Values[5].Bytes),
		string(value.Values[6].Bytes),
	}
	wantArgs := []string{"20", "100", "60000"}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Errorf("BuildCommand() argument %d = %q, want %q", i, gotArgs[i], wantArgs[i])
		}
	}
}

func TestParseResultReturnsNonZeroResetForAllowedRequest(t *testing.T) {
	request := Request{Key: "limit-key", Requests: 100, Window: time.Minute, Capacity: 20}
	result, err := ParseResult(request, []byte("*5\r\n:1\r\n:1\r\n:20\r\n:19\r\n:3000\r\n"))
	if err != nil {
		t.Fatalf("ParseResult() error = %v", err)
	}
	if !result.Allowed || result.Limit != 20 || result.Remaining != 19 || result.ResetSeconds != 3 {
		t.Errorf("ParseResult() = %+v, want allowed result with limit 20, remaining 19 and reset 3", result)
	}
}

func TestBuildCommandRejectsMissingCapacity(t *testing.T) {
	_, err := BuildCommand(Request{Key: "limit-key", Requests: 100, Window: time.Minute})
	if err == nil {
		t.Fatal("BuildCommand(Capacity=0) error = nil, want error")
	}
}
