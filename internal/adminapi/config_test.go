package adminapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lgc202/ingate/internal/adminapi/conf"
)

func TestExampleConfig(t *testing.T) {
	config, err := loadConfig("../../configs/admin-api.yaml")
	if err != nil {
		t.Fatalf("loadConfig(configs/admin-api.yaml) error = %v, want nil", err)
	}
	if config.GetApiServer().GetKubeconfig() == "" {
		t.Error("loadConfig(configs/admin-api.yaml) kubeconfig is empty, want example path")
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
