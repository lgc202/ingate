package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelpAndVersionWithoutConfig(t *testing.T) {
	for _, option := range []string{"--help", "--version"} {
		t.Run(option, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			configFile := filepath.Join(t.TempDir(), "missing.yaml")

			code := run([]string{option, "--conf", configFile}, &stdout, &stderr)

			if code != 0 || stdout.Len()+stderr.Len() == 0 {
				t.Errorf("run(%s) exit/output = %d/%q/%q, want success without reading config", option, code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunRejectsUnknownFlagAndMissingConfig(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want int
	}{
		{name: "unknown flag", args: []string{"--unknown"}, want: 2},
		{name: "missing config", args: []string{"--conf", filepath.Join(t.TempDir(), "missing.yaml")}, want: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run(tt.args, &stdout, &stderr)

			if code != tt.want || stdout.Len() != 0 || !strings.Contains(stderr.String(), "ingate-ai-proxy") {
				t.Errorf("run(%s) exit/output = %d/%q/%q, want %d with diagnostic", tt.name, code, stdout.String(), stderr.String(), tt.want)
			}
		})
	}
}
