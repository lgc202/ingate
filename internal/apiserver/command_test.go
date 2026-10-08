package apiserver

import (
	"bytes"
	"testing"

	"github.com/lgc202/ingate/internal/pkg/version"
)

func TestNewCommand(t *testing.T) {
	options, err := NewOptions()
	if err != nil {
		t.Fatalf("NewOptions() error = %v, want nil", err)
	}
	cmd := NewCommand(t.Context(), options)

	if cmd.Use != componentName {
		t.Errorf("NewCommand() Use = %q, want %q", cmd.Use, componentName)
	}
	for _, name := range []string{"etcd-servers", "secure-port", "authentication-kubeconfig"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("NewCommand() flag %q = nil, want registered", name)
		}
	}
	if err := cmd.Args(cmd, []string{"unexpected"}); err == nil {
		t.Error("NewCommand() Args(unexpected) error = nil, want non-nil")
	}
}

func TestNewCommandValidatesBeforeRun(t *testing.T) {
	options, err := NewOptions()
	if err != nil {
		t.Fatalf("NewOptions() error = %v, want nil", err)
	}
	options.RecommendedOptions.SecureServing.BindPort = -1
	cmd := NewCommand(t.Context(), options)

	if err := cmd.RunE(cmd, nil); err == nil {
		t.Error("NewCommand().RunE(invalid bind port) error = nil, want non-nil")
	}
}

func TestNewCommandVersionSkipsValidationAndRun(t *testing.T) {
	options, err := NewOptions()
	if err != nil {
		t.Fatalf("NewOptions() error = %v, want nil", err)
	}
	cmd := NewCommand(t.Context(), options)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--version", "--secure-port=-1"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("Execute(--version, invalid port) error = %v, want nil", err)
	}
	if got, want := stdout.String(), version.Get().Text()+"\n"; got != want {
		t.Errorf("Execute(--version) stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("Execute(--version) stderr = %q, want empty", stderr.String())
	}
}
