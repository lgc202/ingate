package apiserver

import "testing"

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
