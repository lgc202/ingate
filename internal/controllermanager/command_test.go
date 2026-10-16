package controllermanager

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/lgc202/ingate/internal/pkg/version"
)

func TestNewCommandVersionSkipsConfigurationAndRun(t *testing.T) {
	cmd := NewCommand(t.Context(), NewOptions())
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--version", "--config", filepath.Join(t.TempDir(), "missing.yaml")})
	if err := cmd.Execute(); err != nil {
		t.Errorf("Execute(--version, missing config) error = %v, want nil", err)
	}

	if got, want := stdout.String(), version.Get().Text()+"\n"; got != want {
		t.Errorf("Execute(--version) stdout = %q, want %q", got, want)
	}

	if stderr.Len() != 0 {
		t.Errorf("Execute(--version) stderr = %q, want empty", stderr.String())
	}
}
