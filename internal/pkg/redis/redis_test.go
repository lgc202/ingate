package redis

import (
	"crypto/tls"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	goredis "github.com/redis/go-redis/v9"
)

func TestNewSelectsNativeClient(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantCluster bool
	}{
		{name: "single node", config: Config{Addrs: []string{"127.0.0.1:0"}, DB: 3}},
		{name: "Sentinel takes precedence over multiple addresses", config: Config{Addrs: []string{"127.0.0.1:0", "127.0.0.1:1"}, MasterName: "ingate", DB: 3}},
		{name: "multiple Cluster seeds", config: Config{Addrs: []string{"127.0.0.1:0", "127.0.0.1:1"}}, wantCluster: true},
		{name: "single Cluster seed", config: Config{Addrs: []string{"127.0.0.1:0"}, IsClusterMode: true}, wantCluster: true},
		{name: "Sentinel FailoverClusterClient", config: Config{Addrs: []string{"127.0.0.1:0"}, MasterName: "ingate", IsClusterMode: true, DB: 3}, wantCluster: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config)
			if err != nil {
				t.Fatalf("New() error = %v, want nil", err)
			}

			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Errorf("Close() error = %v, want nil", err)
				}
			})

			if tt.wantCluster {
				cluster, ok := client.(*goredis.ClusterClient)
				if !ok {
					t.Fatalf("New() type = %T, want *redis.ClusterClient", client)
				}

				options := cluster.Options()
				if options.ReadOnly || options.RouteByLatency || options.RouteRandomly {
					t.Error("New() enabled replica routing for sessions")
				}

				if tt.config.MasterName != "" && tt.config.DB != 0 && options.OnConnect == nil {
					t.Error("Sentinel FailoverClusterClient has no SELECT hook for nonzero db")
				}

				return
			}

			single, ok := client.(*goredis.Client)
			if !ok {
				t.Fatalf("New() type = %T, want *redis.Client", client)
			}

			if got := single.Options().DB; got != tt.config.DB {
				t.Errorf("New() db = %d, want %d", got, tt.config.DB)
			}
		})
	}
}

func TestNewDoesNotProbeUnavailableServer(t *testing.T) {
	server := miniredis.RunT(t)
	addr := server.Addr()
	server.Close()

	client, err := New(Config{Addrs: []string{addr}})
	if err != nil {
		t.Fatalf("New(unavailable server) error = %v, want nil before startup probe", err)
	}

	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestOptionsPreserveConfiguredFields(t *testing.T) {
	config := Config{
		Addrs:    []string{"redis-1:26379", "redis-2:26379"},
		Username: "data-user", Password: "data-password", SentinelUsername: "sentinel-user", SentinelPassword: "sentinel-password",
		DB: 3, MasterName: "ingate", IsClusterMode: true,
		PoolSize: 7, MinIdleConns: 2,
		DialTimeout: 3 * time.Second, ReadTimeout: 4 * time.Second, WriteTimeout: 5 * time.Second, PoolTimeout: 6 * time.Second,
		TLS: true, TLSServerName: "redis.example.com",
	}
	want := &goredis.UniversalOptions{
		Addrs:    []string{"redis-1:26379", "redis-2:26379"},
		Username: "data-user", Password: "data-password", SentinelUsername: "sentinel-user", SentinelPassword: "sentinel-password",
		DB: 3, MasterName: "ingate", IsClusterMode: true,
		PoolSize: 7, MinIdleConns: 2,
		DialTimeout: 3 * time.Second, ReadTimeout: 4 * time.Second, WriteTimeout: 5 * time.Second, PoolTimeout: 6 * time.Second,
		ContextTimeoutEnabled: true,
	}
	got := config.options()
	if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(goredis.UniversalOptions{}, "TLSConfig")); diff != "" {
		t.Errorf("options() mismatch (-want +got):\n%s", diff)
	}

	if got.TLSConfig == nil {
		t.Fatal("options() TLSConfig = nil, want certificate verification enabled")
	}

	if got.TLSConfig.MinVersion != tls.VersionTLS12 || got.TLSConfig.ServerName != "redis.example.com" || got.TLSConfig.InsecureSkipVerify || got.TLSConfig.RootCAs != nil {
		t.Error("options() TLS settings do not use TLS 1.2 minimum, requested name and system certificate verification")
	}
}

func TestOptionsPreserveDriverDefaults(t *testing.T) {
	config := Config{Addrs: []string{"redis:6379"}}
	want := &goredis.UniversalOptions{Addrs: []string{"redis:6379"}, ContextTimeoutEnabled: true}
	if diff := cmp.Diff(want, config.options()); diff != "" {
		t.Errorf("options() injected driver settings (-want +got):\n%s", diff)
	}
}

func TestClientDoesNotBorrowAddressSlice(t *testing.T) {
	config := Config{Addrs: []string{"127.0.0.1:0", "127.0.0.1:1"}}
	client, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
	})
	cluster, ok := client.(*goredis.ClusterClient)
	if !ok {
		t.Fatalf("New() type = %T, want *redis.ClusterClient", client)
	}

	config.Addrs[0] = "changed:6379"
	want := []string{"127.0.0.1:0", "127.0.0.1:1"}
	if diff := cmp.Diff(want, cluster.Options().Addrs); diff != "" {
		t.Errorf("client addresses changed with caller config (-want +got):\n%s", diff)
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{name: "no addresses", wantErr: "addresses are required"},
		{name: "empty address", config: Config{Addrs: []string{" "}}, wantErr: "address at index 0 is empty"},
		{name: "negative db", config: Config{Addrs: []string{"redis:6379"}, DB: -1}, wantErr: "db must be non-negative"},
		{name: "Cluster db with multiple seeds", config: Config{Addrs: []string{"redis-1:6379", "redis-2:6379"}, DB: 1}, wantErr: "cluster requires db 0"},
		{name: "Cluster db with single seed", config: Config{Addrs: []string{"redis:6379"}, IsClusterMode: true, DB: 1}, wantErr: "cluster requires db 0"},
		{name: "Sentinel username without master", config: Config{Addrs: []string{"redis:6379"}, SentinelUsername: "user"}, wantErr: "credentials require master_name"},
		{name: "Sentinel password without master", config: Config{Addrs: []string{"redis:6379"}, SentinelPassword: "secret"}, wantErr: "credentials require master_name"},
		{name: "negative pool size", config: Config{Addrs: []string{"redis:6379"}, PoolSize: -1}, wantErr: "pool sizes must be non-negative"},
		{name: "negative idle connections", config: Config{Addrs: []string{"redis:6379"}, MinIdleConns: -1}, wantErr: "pool sizes must be non-negative"},
		{name: "negative dial timeout", config: Config{Addrs: []string{"redis:6379"}, DialTimeout: -time.Second}, wantErr: "dial_timeout must be non-negative"},
		{name: "negative read timeout", config: Config{Addrs: []string{"redis:6379"}, ReadTimeout: -time.Second}, wantErr: "read_timeout must be non-negative"},
		{name: "negative write timeout", config: Config{Addrs: []string{"redis:6379"}, WriteTimeout: -time.Second}, wantErr: "write_timeout must be non-negative"},
		{name: "negative pool timeout", config: Config{Addrs: []string{"redis:6379"}, PoolTimeout: -time.Second}, wantErr: "pool_timeout must be non-negative"},
		{name: "TLS name without TLS", config: Config{Addrs: []string{"redis:6379"}, TLSServerName: "redis.example.com"}, wantErr: "tls_server_name requires tls"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config)
			if client != nil {
				if closeErr := client.Close(); closeErr != nil {
					t.Errorf("Close() error = %v, want nil", closeErr)
				}

				t.Error("New() client is not nil, want nil")
			}

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("New() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
