package controllermanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExampleConfig(t *testing.T) {
	config, err := loadConfig("../../configs/controller-manager.yaml")
	if err != nil {
		t.Fatalf("loadConfig(example) error = %v, want nil", err)
	}

	if config.GetMysql().GetDsn() == "" {
		t.Error("loadConfig(example) MySQL DSN is empty")
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
