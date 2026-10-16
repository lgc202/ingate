package adminapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"

	apiserverstore "github.com/lgc202/ingate/internal/adminapi/repository/apiserver"
	"github.com/lgc202/ingate/internal/pkg/version"
)

func TestNewAppUsesVersionAndConfiguredLogger(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	config, err := loadConfig("../../configs/admin-api.yaml")
	if err != nil {
		t.Fatalf("loadConfig(example) error = %v, want nil", err)
	}

	apiStore, err := apiserverstore.New(writeTestKubeconfig(t))
	if err != nil {
		t.Fatalf("apiserverstore.New(test kubeconfig) error = %v, want nil", err)
	}

	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil)).With("service", componentName)
	app := newApp(t.Context(), config, apiStore, nil, nil, logger)
	if got, want := app.Version(), version.Get().String(); got != want {
		t.Errorf("newApp().Version() = %q, want %q", got, want)
	}

	if slog.Default() != logger {
		t.Error("newApp() default Logger differs from injected Logger")
	}

	slog.Info("test assembly")
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("assembled logger output = %q, error = %v, want JSON", output.String(), err)
	}

	if record["service"] != componentName || record["msg"] != "test assembly" {
		t.Errorf("assembled logger record = %v, want component service and test assembly message", record)
	}
}

func TestRunPreservesStartupError(t *testing.T) {
	data, err := os.ReadFile("../../configs/admin-api.yaml")
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}

	data = bytes.Replace(data, []byte("ingate:change-me@tcp(127.0.0.1:3306)/ingate"), []byte("invalid-dsn"), 1)
	data = bytes.Replace(data, []byte("/etc/ingate/admin-api.kubeconfig"), []byte(writeTestKubeconfig(t)), 1)
	path := filepath.Join(t.TempDir(), "admin-api.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write failed startup config: %v", err)
	}

	_, wantCause := mysqldriver.ParseDSN("invalid-dsn")
	if wantCause == nil {
		t.Fatal("ParseDSN(invalid-dsn) error = nil, want invalid DSN")
	}

	if err := Run(t.Context(), path); !errors.Is(err, wantCause) {
		t.Errorf("Run(invalid DSN) error = %v, want original DSN cause %v", err, wantCause)
	}
}

func TestRunRejectsKubeconfigBeforeOpeningDatabase(t *testing.T) {
	data, err := os.ReadFile("../../configs/admin-api.yaml")
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}

	missing := filepath.Join(t.TempDir(), "missing.kubeconfig")
	data = bytes.Replace(data, []byte("/etc/ingate/admin-api.kubeconfig"), []byte(missing), 1)
	// 即使启动顺序回归，也只会遇到 DSN 解析错误，不会连接真实数据库。
	data = bytes.Replace(data, []byte("ingate:change-me@tcp(127.0.0.1:3306)/ingate"), []byte("invalid-dsn"), 1)
	configFile := filepath.Join(t.TempDir(), "admin-api.yaml")
	if err := os.WriteFile(configFile, data, 0o600); err != nil {
		t.Fatalf("write failed startup config: %v", err)
	}

	if err := Run(t.Context(), configFile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Run(missing kubeconfig) error = %v, want file error before MySQL initialization", err)
	}
}

func writeTestKubeconfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	data := `apiVersion: v1
kind: Config
clusters:
  - name: test
    cluster:
      server: https://127.0.0.1:1
contexts:
  - name: test
    context:
      cluster: test
current-context: test
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write test kubeconfig: %v", err)
	}

	return path
}
