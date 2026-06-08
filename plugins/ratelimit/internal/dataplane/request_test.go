package dataplane

import (
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
)

func TestNewCheckRequestUsesReferencedRedisStore(t *testing.T) {
	request, err := NewCheckRequest([]config.RedisStore{
		{Name: "redis-main", Address: "127.0.0.1:6379"},
	}, []policy.GlobalCheck{
		{
			Policy:        config.Policy{Name: "policy-a"},
			Rule:          config.Rule{Name: "tenant", Algorithm: config.AlgorithmFixedWindow},
			RedisStore:    "redis-main",
			RedisKey:      "ingate:test",
			Requests:      10,
			WindowSeconds: 60,
		},
	})
	if err != nil {
		t.Fatalf("NewCheckRequest() error = %v", err)
	}
	if len(request.Checks) != 1 {
		t.Fatalf("len(Checks) = %d, want 1", len(request.Checks))
	}
	if request.Checks[0].RedisStore.ID != "redis-main" {
		t.Fatalf("RedisStore.ID = %q, want redis-main", request.Checks[0].RedisStore.ID)
	}
}

func TestResponseStatusFromHeadersIgnoresMalformedHeaderPair(t *testing.T) {
	status := responseStatusFromHeaders([][2]string{
		{":status", "200"},
	})
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}

	status = responseStatusFromHeaders([][2]string{
		{"content-type", "application/json"},
	})
	if status != 0 {
		t.Fatalf("missing status = %d, want 0", status)
	}
}
