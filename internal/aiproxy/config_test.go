package aiproxy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/lgc202/ingate/internal/aiproxy/conf"
	logconfig "github.com/lgc202/ingate/internal/pkg/log"
)

const validStartupYAML = `server:
  socket: /run/ingate/ingress.sock
  max_request_body_bytes: 1048576
  shutdown_timeout: 30s
api_server:
  kubeconfig: /etc/ingate/ai-proxy.kubeconfig
  ai_config_namespace: control-plane
envoy:
  egress_socket: /run/ingate/egress.sock
`

func TestLoadConfigExample(t *testing.T) {
	want := &conf.Bootstrap{
		Server: &conf.Server{
			Socket:              "/run/ingate/ingress.sock",
			MaxRequestBodyBytes: 1048576,
			ShutdownTimeout:     durationpb.New(30 * time.Second),
		},
		ApiServer: &conf.APIServer{
			Kubeconfig: "/etc/ingate/ai-proxy.kubeconfig", AiConfigNamespace: "control-plane",
		},
		Envoy: &conf.Envoy{EgressSocket: "/run/ingate/egress.sock"},
		Log:   &conf.Log{Level: "info", Format: "json", Outputs: []string{"stdout"}},
	}

	config, err := loadConfig("../../configs/ai-proxy.yaml")
	if err != nil {
		t.Fatalf("loadConfig(example) error = %v", err)
	}

	if diff := cmp.Diff(want, config, protocmp.Transform()); diff != "" {
		t.Errorf("loadConfig(example) mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadConfigRejectsInvalidInputs(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
	}{
		{name: "empty file"},
		{name: "invalid YAML", data: "server: ["},
		{name: "unknown field", data: validStartupYAML + "unknown: true\n"},
		{name: "unknown nested field", data: strings.Replace(validStartupYAML, "max_request_body_bytes", "max_request_body_byte", 1)},
		{name: "duplicate section", data: validStartupYAML + "server: {}\n"},
		{name: "duplicate field", data: strings.Replace(validStartupYAML, "  max_request_body_bytes:", "  socket: /run/other.sock\n  max_request_body_bytes:", 1)},
		{name: "duplicate proto field alias", data: strings.Replace(validStartupYAML, "  max_request_body_bytes:", "  maxRequestBodyBytes: 2097152\n  max_request_body_bytes:", 1)},
		{name: "missing server", data: strings.Replace(validStartupYAML, "server:\n  socket: /run/ingate/ingress.sock\n  max_request_body_bytes: 1048576\n  shutdown_timeout: 30s\n", "", 1)},
		{name: "missing API connection", data: strings.Replace(validStartupYAML, "api_server:\n  kubeconfig: /etc/ingate/ai-proxy.kubeconfig\n  ai_config_namespace: control-plane\n", "", 1)},
		{name: "missing Envoy", data: strings.Replace(validStartupYAML, "envoy:\n  egress_socket: /run/ingate/egress.sock\n", "", 1)},
		{name: "zero body limit", data: strings.Replace(validStartupYAML, "1048576", "0", 1)},
		{name: "negative body limit", data: strings.Replace(validStartupYAML, "1048576", "-1", 1)},
		{name: "missing shutdown timeout", data: strings.Replace(validStartupYAML, "  shutdown_timeout: 30s\n", "", 1)},
		{name: "zero shutdown timeout", data: strings.Replace(validStartupYAML, "30s", "0s", 1)},
		{name: "negative shutdown timeout", data: strings.Replace(validStartupYAML, "30s", "-1s", 1)},
		{name: "missing body limit", data: strings.Replace(validStartupYAML, "  max_request_body_bytes: 1048576\n", "", 1)},
		{name: "overflow body limit", data: strings.Replace(validStartupYAML, "1048576", "9223372036854775808", 1)},
		{name: "empty namespace", data: strings.Replace(validStartupYAML, "control-plane", "''", 1)},
		{name: "invalid namespace", data: strings.Replace(validStartupYAML, "control-plane", "Control-Plane", 1)},
		{name: "relative ingress", data: strings.Replace(validStartupYAML, "/run/ingate/ingress.sock", "ingress.sock", 1)},
		{name: "relative egress", data: strings.Replace(validStartupYAML, "/run/ingate/egress.sock", "egress.sock", 1)},
		{name: "empty ingress", data: strings.Replace(validStartupYAML, "/run/ingate/ingress.sock", "''", 1)},
		{name: "empty egress", data: strings.Replace(validStartupYAML, "/run/ingate/egress.sock", "''", 1)},
		{name: "NUL ingress", data: strings.Replace(validStartupYAML, "/run/ingate/ingress.sock", `"/run/invalid\0.sock"`, 1)},
		{name: "NUL egress", data: strings.Replace(validStartupYAML, "/run/ingate/egress.sock", `"/run/invalid\0.sock"`, 1)},
		{name: "same socket", data: strings.Replace(validStartupYAML, "/run/ingate/egress.sock", "/run/ingate/./ingress.sock", 1)},
		{name: "unknown log level", data: validStartupYAML + "log: {level: verbose}\n"},
		{name: "unknown log format", data: validStartupYAML + "log: {format: xml}\n"},
		{name: "unknown log output", data: validStartupYAML + "log: {outputs: [remote]}\n"},
		{name: "duplicate log output", data: validStartupYAML + "log: {outputs: [stdout, stdout]}\n"},
		{name: "missing log file", data: validStartupYAML + "log: {outputs: [file]}\n"},
		{name: "empty log file path", data: validStartupYAML + "log: {outputs: [file], file: {path: ''}}\n"},
		{name: "negative log file size", data: validStartupYAML + "log: {file: {max_size_mb: -1}}\n"},
		{name: "negative log file backups", data: validStartupYAML + "log: {file: {max_backups: -1}}\n"},
		{name: "negative log file age", data: validStartupYAML + "log: {file: {max_age_days: -1}}\n"},
		{name: "unknown log field", data: validStartupYAML + "log: {levle: info}\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			configFile := writeStartupConfig(t, tt.data)

			config, err := loadConfig(configFile)
			if err == nil || config != nil {
				t.Fatalf("loadConfig(%s) = (%v, %v), want nil configuration and error", tt.name, config, err)
			}

			if !strings.Contains(err.Error(), configFile) {
				t.Errorf("loadConfig(%s) error = %v, want file context %q", tt.name, err, configFile)
			}
		})
	}
}

func TestLoadConfigDoesNotAcquireRuntimeResources(t *testing.T) {
	directory := t.TempDir()
	ingress := filepath.Join(directory, "not-created", "ingress.sock")
	kubeconfig := filepath.Join(directory, "missing.kubeconfig")
	data := strings.ReplaceAll(validStartupYAML, "/run/ingate/ingress.sock", ingress)
	data = strings.ReplaceAll(data, "/etc/ingate/ai-proxy.kubeconfig", kubeconfig)
	data += "log:\n  outputs: [file]\n  file:\n    path: " + filepath.Join(directory, "not-created", "ai-proxy.log") + "\n"
	configFile := writeStartupConfig(t, data)

	config, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("loadConfig(unallocated resources) error = %v", err)
	}

	if config.GetServer().GetSocket() != ingress || config.GetApiServer().GetKubeconfig() != kubeconfig {
		t.Errorf("loadConfig(unallocated resources) changed configured paths: %v", config)
	}

	if _, err := os.Stat(filepath.Dir(ingress)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("loadConfig(unallocated resources) directory error = %v, want untouched filesystem", err)
	}
}

func TestLoadConfigAllowsInClusterConfiguration(t *testing.T) {
	data := strings.Replace(validStartupYAML, "  kubeconfig: /etc/ingate/ai-proxy.kubeconfig\n", "", 1)
	configFile := writeStartupConfig(t, data)

	config, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("loadConfig(in-cluster) error = %v", err)
	}

	if config.GetApiServer().GetKubeconfig() != "" {
		t.Errorf("loadConfig(in-cluster) kubeconfig = %q, want empty path without API access", config.GetApiServer().GetKubeconfig())
	}
}

func TestLoadConfigPreservesReadFailure(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "missing.yaml")

	config, err := loadConfig(configFile)
	if !errors.Is(err, os.ErrNotExist) || config != nil {
		t.Errorf("loadConfig(missing file) = (%v, %v), want nil and os.ErrNotExist", config, err)
	}
}

func TestLoadConfigLogOptions(t *testing.T) {
	data := validStartupYAML + `log:
  level: warn
  format: text
  outputs: [stderr, stdout, file]
  add_source: true
  file:
    path: ai-proxy.log
    max_size_mb: 11
    max_backups: 3
    max_age_days: 4
    compress: true
`
	configFile := writeStartupConfig(t, data)
	want := logconfig.Config{
		Level:     "warn",
		Format:    logconfig.FormatText,
		Outputs:   []logconfig.Output{logconfig.OutputStderr, logconfig.OutputStdout, logconfig.OutputFile},
		AddSource: true,
		File: logconfig.FileConfig{
			Path:       "ai-proxy.log",
			MaxSizeMB:  11,
			MaxBackups: 3,
			MaxAgeDays: 4,
			Compress:   true,
		},
	}

	config, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("loadConfig(log options) error = %v, want nil", err)
	}

	if diff := cmp.Diff(want, logConfig(config.GetLog())); diff != "" {
		t.Errorf("logConfig(loaded options) mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadConfigLogDefaults(t *testing.T) {
	for _, suffix := range []string{"", "log: {}\n"} {
		configFile := writeStartupConfig(t, validStartupYAML+suffix)

		config, err := loadConfig(configFile)
		if err != nil {
			t.Fatalf("loadConfig(%q) error = %v, want nil", suffix, err)
		}

		// 默认值由共享日志包确定；组件转换不覆盖默认值或制造文件输出。
		if diff := cmp.Diff(logconfig.Config{}, logConfig(config.GetLog())); diff != "" {
			t.Errorf("logConfig(%q) mismatch (-want +got):\n%s", suffix, diff)
		}
	}
}

func writeStartupConfig(t *testing.T, data string) string {
	t.Helper()
	configFile := filepath.Join(t.TempDir(), "ai-proxy.yaml")
	if err := os.WriteFile(configFile, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile(startup configuration) error = %v", err)
	}

	return configFile
}
