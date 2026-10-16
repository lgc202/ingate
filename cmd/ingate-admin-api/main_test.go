package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lgc202/ingate/internal/pkg/version"
)

type failingWriter struct{}

func TestRunVersionSkipsConfigurationAndMigration(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	for _, args := range [][]string{
		{"--version"},
		{"--version", "-conf", missing, "-migrate"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != 0 {
				t.Errorf("run(%v) exit code = %d, want 0; stderr=%q", args, code, stderr.String())
			}

			if got, want := stdout.String(), version.Get().Text()+"\n"; got != want {
				t.Errorf("run(%v) stdout = %q, want %q", args, got, want)
			}

			if stderr.Len() != 0 {
				t.Errorf("run(%v) stderr = %q, want empty", args, stderr.String())
			}
		})
	}
}

func TestRunVersionFalseStillLoadsConfiguration(t *testing.T) {
	args := []string{"--version=false", "-conf", filepath.Join(t.TempDir(), "missing.yaml")}
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != 1 {
		t.Errorf("run(%v) exit code = %d, want 1 for missing config", args, code)
	}

	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "load Admin API config") {
		t.Errorf("run(%v) stdout/stderr = %q/%q, want empty stdout and config error", args, stdout.String(), stderr.String())
	}
}

func TestRunFlagExitCodes(t *testing.T) {
	tests := []struct {
		args     []string
		wantCode int
		wantText string
	}{
		{args: []string{"-help"}, wantText: "Usage of ingate-admin-api"},
		{args: []string{"-unknown"}, wantCode: 2, wantText: "flag provided but not defined"},
		{args: []string{"--version=invalid"}, wantCode: 2, wantText: "invalid boolean value"},
		{args: []string{"unused"}, wantCode: 2, wantText: "unexpected positional arguments"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr); code != tt.wantCode {
				t.Errorf("run(%v) exit code = %d, want %d", tt.args, code, tt.wantCode)
			}

			if stdout.Len() != 0 || !strings.Contains(stderr.String(), tt.wantText) {
				t.Errorf("run(%v) stdout/stderr = %q/%q, want empty stdout and %q", tt.args, stdout.String(), stderr.String(), tt.wantText)
			}
		})
	}
}

func TestRunVersionOutputFailure(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"--version"}, failingWriter{}, &stderr); code != 1 {
		t.Errorf("run(--version, failing output) exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "print version") {
		t.Errorf("run(--version, failing output) stderr = %q, want version output error", stderr.String())
	}
}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}
