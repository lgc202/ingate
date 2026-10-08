package adminapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	logconfig "github.com/lgc202/ingate/internal/pkg/log"
	redisclient "github.com/lgc202/ingate/internal/pkg/redis"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestExampleConfig(t *testing.T) {
	config, err := loadConfig("../../configs/admin-api.yaml")
	if err != nil {
		t.Fatalf("loadConfig(configs/admin-api.yaml) error = %v, want nil", err)
	}
	if config.GetApiServer().GetKubeconfig() == "" {
		t.Error("loadConfig(configs/admin-api.yaml) kubeconfig is empty, want example path")
	}
	if diff := cmp.Diff([]string{"127.0.0.1:6379"}, config.GetRedis().GetAddrs()); diff != "" {
		t.Errorf("example Redis addresses mismatch (-want +got):\n%s", diff)
	}
	wantLog := logconfig.Config{Level: "info", Format: logconfig.FormatJSON, Outputs: []logconfig.Output{logconfig.OutputStdout}}
	if diff := cmp.Diff(wantLog, logConfig(config.GetLog())); diff != "" {
		t.Errorf("example log config mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadConfigLogModesAndInvalidInputs(t *testing.T) {
	tests := []struct {
		name    string
		log     string
		wantErr bool
	}{
		{name: "omitted"},
		{name: "empty", log: "log: {}\n"},
		{name: "multiple outputs", log: "log:\n  level: debug\n  format: text\n  outputs: [stdout, stderr]\n  add_source: true\n"},
		{name: "unknown level", log: "log:\n  level: verbose\n", wantErr: true},
		{name: "unknown format", log: "log:\n  format: xml\n", wantErr: true},
		{name: "duplicate output", log: "log:\n  outputs: [stdout, stdout]\n", wantErr: true},
		{name: "unknown output", log: "log:\n  outputs: [remote]\n", wantErr: true},
		{name: "file output missing config", log: "log:\n  outputs: [file]\n", wantErr: true},
		{name: "file output missing path", log: "log:\n  outputs: [file]\n  file: {}\n", wantErr: true},
		{name: "file output with defaults", log: "log:\n  outputs: [file]\n  file: {path: app.log}\n"},
		{name: "file rotation options", log: "log:\n  outputs: [stdout, file]\n  file:\n    path: app.log\n    max_size_mb: 100\n    max_backups: 7\n    max_age_days: 7\n    compress: true\n"},
		{name: "negative file size", log: "log:\n  outputs: [file]\n  file: {path: app.log, max_size_mb: -1}\n", wantErr: true},
		{name: "negative backup count", log: "log:\n  outputs: [file]\n  file: {path: app.log, max_backups: -1}\n", wantErr: true},
		{name: "negative retention", log: "log:\n  outputs: [file]\n  file: {path: app.log, max_age_days: -1}\n", wantErr: true},
		{name: "unknown field", log: "log:\n  levle: info\n", wantErr: true},
		{name: "duplicate field", log: "log:\n  level: info\n  level: error\n", wantErr: true},
	}
	data, err := os.ReadFile("../../configs/admin-api.yaml")
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}
	base := strings.Replace(string(data), "log:\n  level: info\n  format: json\n  outputs: [stdout]\n", "", 1)
	if base == string(data) {
		t.Fatal("example config has no expected log block")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "admin-api.yaml")
			if err := os.WriteFile(path, []byte(base+tt.log), 0o600); err != nil {
				t.Fatalf("write %s log test config: %v", tt.name, err)
			}
			_, err := loadConfig(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadConfig(%s log) error = %v, want error %t", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestLogConfigPreservesFieldsAndDefaults(t *testing.T) {
	input := &conf.Log{
		Level: "warn", Format: "text", Outputs: []string{"stderr", "stdout", "file"}, AddSource: true,
		File: &conf.Log_File{Path: "app.log", MaxSizeMb: 11, MaxBackups: 3, MaxAgeDays: 4, Compress: true},
	}
	want := logconfig.Config{
		Level: "warn", Format: logconfig.FormatText, Outputs: []logconfig.Output{logconfig.OutputStderr, logconfig.OutputStdout, logconfig.OutputFile}, AddSource: true,
		File: logconfig.FileConfig{Path: "app.log", MaxSizeMB: 11, MaxBackups: 3, MaxAgeDays: 4, Compress: true},
	}
	if diff := cmp.Diff(want, logConfig(input)); diff != "" {
		t.Errorf("logConfig(configured) mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(logconfig.Config{}, logConfig(nil)); diff != "" {
		t.Errorf("logConfig(nil) mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadConfigRedisModesAndInvalidInputs(t *testing.T) {
	tests := []struct {
		name    string
		redis   string
		wantErr bool
	}{
		{name: "single node", redis: "  addrs: [127.0.0.1:6379]\n  db: 3\n"},
		{name: "Sentinel", redis: "  addrs: [sentinel-1:26379, sentinel-2:26379]\n  master_name: ingate\n  db: 3\n"},
		{name: "Cluster seeds", redis: "  addrs: [redis-1:6379, redis-2:6379]\n"},
		{name: "single Cluster seed", redis: "  addrs: [redis:6379]\n  is_cluster_mode: true\n"},
		{name: "Sentinel FailoverClusterClient", redis: "  addrs: [sentinel:26379]\n  master_name: ingate\n  is_cluster_mode: true\n  db: 3\n"},
		{name: "no addresses", redis: "  addrs: []\n", wantErr: true},
		{name: "blank address", redis: "  addrs: [' ']\n", wantErr: true},
		{name: "Cluster nonzero db", redis: "  addrs: [redis-1:6379, redis-2:6379]\n  db: 1\n", wantErr: true},
		{name: "negative pool size", redis: "  addrs: [redis:6379]\n  pool_size: -1\n", wantErr: true},
		{name: "Sentinel credentials without master", redis: "  addrs: [redis:6379]\n  sentinel_password: test-secret\n", wantErr: true},
		{name: "TLS name without TLS", redis: "  addrs: [redis:6379]\n  tls_server_name: redis.example.com\n", wantErr: true},
		{name: "zero explicit timeout", redis: "  addrs: [redis:6379]\n  read_timeout: 0s\n", wantErr: true},
		{name: "negative timeout", redis: "  addrs: [redis:6379]\n  read_timeout: -1s\n", wantErr: true},
		{name: "old URL rejected", redis: "  url: redis://127.0.0.1:6379/0\n", wantErr: true},
		{name: "old URL alongside addresses rejected", redis: "  addrs: [redis:6379]\n  url: redis://127.0.0.1:6379/0\n", wantErr: true},
		{name: "replica routing not exposed", redis: "  addrs: [redis:6379]\n  route_randomly: true\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(writeRedisTestConfig(t, tt.redis))
			if (err != nil) != tt.wantErr {
				t.Errorf("loadConfig(%s) error = %v, want error %t", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestRedisConfigPreservesStructuredFields(t *testing.T) {
	input := &conf.Redis{
		Addrs:    []string{"sentinel-1:26379", "sentinel-2:26379"},
		Username: "data-user", Password: "data-password", SentinelUsername: "sentinel-user", SentinelPassword: "sentinel-password",
		Db: 3, MasterName: "ingate", IsClusterMode: true,
		PoolSize: 7, MinIdleConns: 2,
		DialTimeout: durationpb.New(3 * time.Second), ReadTimeout: durationpb.New(4 * time.Second),
		WriteTimeout: durationpb.New(5 * time.Second), PoolTimeout: durationpb.New(6 * time.Second),
		Tls: true, TlsServerName: "redis.example.com",
	}
	want := redisclient.Config{
		Addrs:    []string{"sentinel-1:26379", "sentinel-2:26379"},
		Username: "data-user", Password: "data-password", SentinelUsername: "sentinel-user", SentinelPassword: "sentinel-password",
		DB: 3, MasterName: "ingate", IsClusterMode: true,
		PoolSize: 7, MinIdleConns: 2,
		DialTimeout: 3 * time.Second, ReadTimeout: 4 * time.Second, WriteTimeout: 5 * time.Second, PoolTimeout: 6 * time.Second,
		TLS: true, TLSServerName: "redis.example.com",
	}
	if diff := cmp.Diff(want, redisConfig(input)); diff != "" {
		t.Errorf("redisConfig() mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadConfigRejectsPublicListenAddress(t *testing.T) {
	data, err := os.ReadFile("../../configs/admin-api.yaml")
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}

	configFile := filepath.Join(t.TempDir(), "admin-api.yaml")
	publicConfig := strings.Replace(string(data), "127.0.0.1:8000", "0.0.0.0:8000", 1)
	if err := os.WriteFile(configFile, []byte(publicConfig), 0o600); err != nil {
		t.Fatalf("write public listen config: %v", err)
	}

	if _, err := loadConfig(configFile); err == nil {
		t.Error("loadConfig(public listen address) error = nil, want rejection")
	}
}

func TestListenAddressesRequireLoopback(t *testing.T) {
	tests := []struct {
		name     string
		httpAddr string
		grpcAddr string
		wantErr  bool
	}{
		{name: "IPv4 loopback", httpAddr: "127.0.0.1:8000", grpcAddr: "127.0.0.1:9000"},
		{name: "IPv6 loopback", httpAddr: "[::1]:8000", grpcAddr: "[::1]:9000"},
		{name: "public HTTP", httpAddr: "0.0.0.0:8000", grpcAddr: "127.0.0.1:9000", wantErr: true},
		{name: "public gRPC", httpAddr: "127.0.0.1:8000", grpcAddr: "192.0.2.1:9000", wantErr: true},
		{name: "hostname", httpAddr: "localhost:8000", grpcAddr: "127.0.0.1:9000", wantErr: true},
		{name: "missing host", httpAddr: ":8000", grpcAddr: "127.0.0.1:9000", wantErr: true},
		{name: "missing port", httpAddr: "127.0.0.1", grpcAddr: "127.0.0.1:9000", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &conf.Server{
				Http: &conf.Server_HTTP{Addr: tt.httpAddr},
				Grpc: &conf.Server_GRPC{Addr: tt.grpcAddr},
			}

			err := validateListenAddresses(server)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateListenAddresses(%q, %q) error = %v, want error %t", tt.httpAddr, tt.grpcAddr, err, tt.wantErr)
			}
		})
	}
}

func writeRedisTestConfig(t *testing.T, redis string) string {
	t.Helper()
	data, err := os.ReadFile("../../configs/admin-api.yaml")
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}
	prefix, _, ok := strings.Cut(string(data), "redis:\n")
	if !ok {
		t.Fatal("example config has no Redis block")
	}

	path := filepath.Join(t.TempDir(), "admin-api.yaml")
	if err := os.WriteFile(path, []byte(prefix+"redis:\n"+redis), 0o600); err != nil {
		t.Fatalf("write Redis test config: %v", err)
	}
	return path
}
