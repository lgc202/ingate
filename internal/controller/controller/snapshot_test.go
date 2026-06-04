package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	coreruntime "github.com/lgc202/ingate/internal/core/runtime"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestUpsertRuntimeSnapshotSkipsUnchangedSnapshot(t *testing.T) {
	client := fake.NewSimpleClientset(&resource.RuntimeSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "xds-gateway"},
		Spec: resource.RuntimeSnapshotSpec{
			Target:  "xds",
			Gateway: "gateway",
			Version: "version",
			Config:  runtime.RawExtension{Raw: []byte(`{"clusters":[]}`)},
		},
	})
	ctrl := &Controller{client: client}

	err := ctrl.upsertRuntimeSnapshot(context.Background(), coreruntime.RuntimeSnapshot{
		Target:  "xds",
		Gateway: "gateway",
		Version: "version",
		Config:  map[string]any{"clusters": []any{}},
	})
	if err != nil {
		t.Fatalf("upsertRuntimeSnapshot() error = %v", err)
	}

	for _, action := range client.Actions() {
		if action.Matches("update", "runtimesnapshots") {
			t.Fatalf("upsertRuntimeSnapshot() issued unexpected update action: %#v", action)
		}
	}
}

func TestUpsertRuntimeSnapshotUpdatesChangedSnapshot(t *testing.T) {
	client := fake.NewSimpleClientset(&resource.RuntimeSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "xds-gateway"},
		Spec: resource.RuntimeSnapshotSpec{
			Target:  "xds",
			Gateway: "gateway",
			Version: "old-version",
			Config:  runtime.RawExtension{Raw: []byte(`{"clusters":[]}`)},
		},
	})
	ctrl := &Controller{client: client}

	err := ctrl.upsertRuntimeSnapshot(context.Background(), coreruntime.RuntimeSnapshot{
		Target:  "xds",
		Gateway: "gateway",
		Version: "new-version",
		Config:  map[string]any{"clusters": []any{}},
	})
	if err != nil {
		t.Fatalf("upsertRuntimeSnapshot() error = %v", err)
	}

	updated := false
	for _, action := range client.Actions() {
		if action.Matches("update", "runtimesnapshots") {
			updated = true
			break
		}
	}
	if !updated {
		t.Fatal("upsertRuntimeSnapshot() did not issue update action")
	}
}
