package controllermanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	mysqlclient "github.com/lgc202/ingate/internal/pkg/mysql"
)

func TestExampleConfig(t *testing.T) {
	config, err := loadConfig("../../configs/controller-manager.yaml")
	if err != nil {
		t.Fatalf("loadConfig(example) error = %v, want nil", err)
	}

	if config.GetMysql().GetDsn() == "" {
		t.Error("loadConfig(example) MySQL DSN is empty")
	}

	want := mysqlclient.Config{
		DSN:                   config.GetMysql().GetDsn(),
		MaxOpenConnections:    20,
		MaxIdleConnections:    10,
		ConnectionMaxLifetime: 30 * time.Minute,
		ConnectionMaxIdleTime: 5 * time.Minute,
	}
	if diff := cmp.Diff(want, mysqlConfig(config.GetMysql())); diff != "" {
		t.Errorf("example MySQL client config mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadConfigRejectsInvalidPoolAndProbeSettings(t *testing.T) {
	data, err := os.ReadFile("../../configs/controller-manager.yaml")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		old  string
		new  string
	}{
		{name: "idle exceeds open", old: "max_idle_connections: 10", new: "max_idle_connections: 21"},
		{name: "missing probe timeout", old: "  ping_timeout: 3s\n"},
		{name: "nonpositive probe timeout", old: "ping_timeout: 3s", new: "ping_timeout: 0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := strings.Replace(string(data), tt.old, tt.new, 1)
			if content == string(data) {
				t.Fatal("config fixture did not change")
			}

			path := filepath.Join(t.TempDir(), "controller-manager.yaml")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}

			if _, err := loadConfig(path); err == nil {
				t.Errorf("loadConfig(%s) succeeded, want configuration rejected before startup", tt.name)
			}
		})
	}
}

func TestLoadConfigRejectsMissingMySQL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller-manager.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := loadConfig(path); err == nil {
		t.Error("loadConfig(missing MySQL) error = nil, want validation error")
	}
}
