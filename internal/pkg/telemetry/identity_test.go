package telemetry

import "testing"

// TestNewIdentityCreatesUniqueProcessInstances 验证同一主机上的启动实例具有不同标识。
func TestNewIdentityCreatesUniqueProcessInstances(t *testing.T) {
	first, err := NewIdentity("ingate-als", "test")
	if err != nil {
		t.Fatalf("NewIdentity(first) error: %v", err)
	}
	second, err := NewIdentity("ingate-als", "test")
	if err != nil {
		t.Fatalf("NewIdentity(second) error: %v", err)
	}

	if first.InstanceID == "" {
		t.Error("NewIdentity(first).InstanceID is empty, want a generated ID")
	}
	if first.InstanceID == second.InstanceID {
		t.Errorf("NewIdentity() instance IDs = %q and %q, want distinct IDs", first.InstanceID, second.InstanceID)
	}
	if got, want := first.Namespace, "ingate"; got != want {
		t.Errorf("NewIdentity().Namespace = %q, want %q", got, want)
	}
	if got, want := first.Environment, "test"; got != want {
		t.Errorf("NewIdentity().Environment = %q, want %q", got, want)
	}
}
