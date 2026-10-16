//go:build integration

package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	redisclient "github.com/lgc202/ingate/internal/pkg/redis"
	goredis "github.com/redis/go-redis/v9"
)

// INGATE_REDIS_TEST_CONFIG 是 redisclient.Config 的 JSON；测试只清理自己创建的键。
func TestSessionIntegration(t *testing.T) {
	config := integrationConfig(t)
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	client := newIntegrationClient(t, ctx, config)
	store := New(client)
	prefix := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	ids := []string{prefix + "-deleted", prefix + "-expired", prefix + "-past"}
	cleanupIntegrationSessions(t, store, ids)
	want := Session{AccountID: 17, AccountVersion: 3}

	if _, err := store.GetSession(ctx, ids[0]); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSession(missing) error = %v, want ErrNotFound", err)
	}

	if err := store.CreateSession(ctx, ids[0], want, time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("CreateSession() error = %v, want nil", err)
	}

	got, err := store.GetSession(ctx, ids[0])
	if err != nil || got != want {
		t.Errorf("GetSession() = %+v, %v, want %+v, nil", got, err, want)
	}

	value, err := client.Get(ctx, sessionKey(ids[0])).Result()
	if err != nil || value != `{"account_id":17,"account_version":3}` {
		t.Errorf("raw session = %q, %v, want original JSON fields", value, err)
	}

	if err := store.DeleteSession(ctx, ids[0]); err != nil {
		t.Errorf("DeleteSession() error = %v, want nil", err)
	}

	if _, err := store.GetSession(ctx, ids[0]); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSession(deleted) error = %v, want ErrNotFound", err)
	}

	if err := store.DeleteSession(ctx, ids[0]); err != nil {
		t.Errorf("DeleteSession(missing) error = %v, want nil", err)
	}

	if err := store.CreateSession(ctx, ids[2], want, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("CreateSession(past deadline) error = %v, want nil", err)
	}

	if _, err := store.GetSession(ctx, ids[2]); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSession(past deadline) error = %v, want ErrNotFound", err)
	}

	if err := store.CreateSession(ctx, ids[1], want, time.Now().Add(3*time.Second)); err != nil {
		t.Fatalf("CreateSession(expiring) error = %v, want nil", err)
	}

	if ttl, err := client.PTTL(ctx, sessionKey(ids[1])).Result(); err != nil || ttl <= 0 || ttl > 3*time.Second {
		t.Errorf("session PTTL = %v, %v, want positive TTL within deadline", ttl, err)
	}

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, err := store.GetSession(ctx, ids[1])
		if errors.Is(err, ErrNotFound) {
			return
		}

		if err != nil {
			t.Fatalf("GetSession(expiring) error = %v, want nil or ErrNotFound", err)
		}

		select {
		case <-ctx.Done():
			t.Fatal("session did not expire before test deadline")
		case <-ticker.C:
		}
	}
}

func TestClusterRoutesSessionsAcrossMasters(t *testing.T) {
	config := integrationConfig(t)
	if config.MasterName != "" || (!config.IsClusterMode && len(config.Addrs) == 1) {
		t.Skip("requires Redis Cluster with at least two masters owning slots")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	client := newIntegrationClient(t, ctx, config)
	cluster, ok := client.(*goredis.ClusterClient)
	if !ok {
		t.Fatalf("New() type = %T, want *redis.ClusterClient", client)
	}

	store := New(client)
	ids := make([]string, 24)
	prefix := fmt.Sprintf("integration-cluster-%d", time.Now().UnixNano())
	for i := range ids {
		ids[i] = fmt.Sprintf("%s-%d", prefix, i)
	}

	cleanupIntegrationSessions(t, store, ids)
	masters := make(map[string]bool)

	for i, id := range ids {
		want := Session{AccountID: uint64(i + 1), AccountVersion: 3}
		if err := store.CreateSession(ctx, id, want, time.Now().Add(time.Minute)); err != nil {
			t.Fatalf("CreateSession(%d) error = %v, want nil", i, err)
		}

		master, err := cluster.MasterForKey(ctx, sessionKey(id))
		if err != nil {
			t.Fatalf("MasterForKey(%d) error = %v, want nil", i, err)
		}

		masters[master.Options().Addr] = true
		got, err := store.GetSession(ctx, id)
		if err != nil || got != want {
			t.Errorf("GetSession(%d) = %+v, %v, want %+v, nil", i, got, err, want)
		}

		if err := store.DeleteSession(ctx, id); err != nil {
			t.Errorf("DeleteSession(%d) error = %v, want nil", i, err)
		}

		if _, err := store.GetSession(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("GetSession(deleted %d) error = %v, want ErrNotFound", i, err)
		}
	}

	if len(masters) < 2 {
		t.Errorf("sessions covered %d masters, want at least two", len(masters))
	}
}

func TestIntegrationRejectsInvalidCredentials(t *testing.T) {
	config := integrationConfig(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	newIntegrationClient(t, ctx, config)

	tests := []struct {
		name   string
		config redisclient.Config
	}{
		{name: "data node", config: config},
		{name: "Sentinel", config: config},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Sentinel" {
				if tt.config.MasterName == "" || tt.config.SentinelPassword == "" {
					t.Skip("requires password-protected Sentinel")
				}

				tt.config.SentinelPassword += "-wrong"
			} else {
				if tt.config.Password == "" {
					t.Skip("requires password-protected Redis")
				}

				tt.config.Password += "-wrong"
			}

			client, err := redisclient.New(tt.config)
			if err != nil {
				t.Fatalf("New() error = %v, want nil before authentication", err)
			}

			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Errorf("Close() error = %v, want nil", err)
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			if err := client.Ping(ctx).Err(); err == nil || !strings.Contains(err.Error(), "WRONGPASS") {
				t.Errorf("Ping(wrong password) error = %v, want Redis WRONGPASS", err)
			}
		})
	}
}

func TestIntegrationNetworkReadUsesContextDeadline(t *testing.T) {
	config := integrationConfig(t)
	setupCtx, setupCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer setupCancel()
	client := newIntegrationClient(t, setupCtx, config)
	id := fmt.Sprintf("integration-blocked-%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := client.BLPop(ctx, 0, sessionKey(id)).Result()
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		if timeoutErr, ok := errors.AsType[net.Error](err); !ok || !timeoutErr.Timeout() {
			t.Errorf("BLPop() error = %v, want deadline or network timeout", err)
		}
	}

	if elapsed > time.Second {
		t.Errorf("BLPop() took %v, want network deadline to stop it within 1s", elapsed)
	}
}

func integrationConfig(t *testing.T) redisclient.Config {
	t.Helper()
	raw := os.Getenv("INGATE_REDIS_TEST_CONFIG")
	if raw == "" {
		t.Skip("set INGATE_REDIS_TEST_CONFIG to test a dedicated Redis deployment")
	}

	var config redisclient.Config
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		t.Fatal("invalid INGATE_REDIS_TEST_CONFIG JSON")
	}

	return config
}

func newIntegrationClient(t *testing.T, ctx context.Context, config redisclient.Config) goredis.UniversalClient {
	t.Helper()
	client, err := redisclient.New(config)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
	})
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Ping() error = %v, want nil", err)
	}

	return client
}

func cleanupIntegrationSessions(t *testing.T, store *Store, ids []string) {
	t.Helper()
	t.Cleanup(func() {
		// 测试 Context 在 Cleanup 前已取消；清理使用独立、有界的 Context。
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, id := range ids {
			if err := store.DeleteSession(ctx, id); err != nil {
				t.Errorf("clean up test session error = %v, want nil", err)
			}
		}
	})
}
