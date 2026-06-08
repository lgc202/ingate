package ratelimit

import (
	"context"
	"log/slog"
	"testing"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
)

func TestCheckReturnsPerCheckErrorForInvalidRequest(t *testing.T) {
	service := NewService(slog.Default())
	response := service.Check(context.Background(), dataplaneratelimit.CheckRequest{
		Checks: []dataplaneratelimit.Check{
			{
				PolicyName: "policy-a",
				RuleName:   "tenant",
				Limit: dataplaneratelimit.Limit{
					Requests:      1,
					WindowSeconds: 60,
				},
			},
		},
	})

	if len(response.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(response.Results))
	}
	result := response.Results[0]
	if result.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if result.ErrorCode != dataplaneratelimit.ErrorCodeInvalidRequest {
		t.Fatalf("ErrorCode = %q, want %q", result.ErrorCode, dataplaneratelimit.ErrorCodeInvalidRequest)
	}
}

func TestCheckReturnsStableErrorCodeForUnsupportedAlgorithm(t *testing.T) {
	service := NewService(slog.Default())
	response := service.Check(context.Background(), dataplaneratelimit.CheckRequest{
		Checks: []dataplaneratelimit.Check{
			{
				PolicyName: "policy-a",
				RuleName:   "tenant",
				RedisKey:   "ingate:test",
				RedisStore: dataplaneratelimit.RedisStore{ID: "redis-main", Address: "127.0.0.1:6379"},
				Algorithm:  dataplaneratelimit.Algorithm("Unknown"),
				Limit: dataplaneratelimit.Limit{
					Requests:      1,
					WindowSeconds: 60,
				},
			},
		},
	})

	if len(response.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(response.Results))
	}
	result := response.Results[0]
	if result.ErrorCode != dataplaneratelimit.ErrorCodeUnsupportedAlgorithm {
		t.Fatalf("ErrorCode = %q, want %q", result.ErrorCode, dataplaneratelimit.ErrorCodeUnsupportedAlgorithm)
	}
}
