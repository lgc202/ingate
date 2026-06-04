package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	coreruntime "github.com/lgc202/ingate/internal/core/runtime"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func (c *Controller) upsertRuntimeSnapshot(ctx context.Context, snapshot coreruntime.RuntimeSnapshot) error {
	config, err := json.Marshal(snapshot.Config)
	if err != nil {
		return fmt.Errorf("marshal runtime snapshot config: %w", err)
	}
	spec := resource.RuntimeSnapshotSpec{
		Target:  snapshot.Target,
		Gateway: snapshot.Gateway,
		Version: snapshot.Version,
		Config:  runtime.RawExtension{Raw: config},
	}

	name := runtimeSnapshotName(snapshot.Target, snapshot.Gateway)
	runtimeSnapshots := c.client.GatewayV1().RuntimeSnapshots()
	existing, err := runtimeSnapshots.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		_, err = runtimeSnapshots.Create(ctx, &resource.RuntimeSnapshot{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       spec,
		}, metav1.CreateOptions{})
		return err
	}
	if existing.Spec.Target == spec.Target &&
		existing.Spec.Gateway == spec.Gateway &&
		existing.Spec.Version == spec.Version &&
		bytes.Equal(existing.Spec.Config.Raw, spec.Config.Raw) {
		return nil
	}

	updated := existing.DeepCopy()
	updated.Spec = spec
	_, err = runtimeSnapshots.Update(ctx, updated, metav1.UpdateOptions{})
	return err
}

func (c *Controller) deleteRuntimeSnapshot(ctx context.Context, target, gateway string) error {
	name := runtimeSnapshotName(target, gateway)
	err := c.client.GatewayV1().RuntimeSnapshots().Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func runtimeSnapshotName(target, gateway string) string {
	return fmt.Sprintf("%s-%s", target, gateway)
}
