package server

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/cache"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func (s *Server) registerEventHandlers() error {
	_, err := s.factory.Gateway().V1().RuntimeSnapshots().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			s.applySnapshotObject(obj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			s.applySnapshotObject(newObj)
		},
		DeleteFunc: func(obj any) {
			s.deleteSnapshotObject(obj)
		},
	})
	return err
}

func (s *Server) waitForCacheSync(ctx context.Context) error {
	for _, synced := range s.factory.WaitForCacheSync(ctx.Done()) {
		if synced {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("cache sync failed")
	}
	return nil
}

func (s *Server) applySnapshotObject(obj any) {
	snapshot, ok := objectAs[*resource.RuntimeSnapshot](obj)
	if !ok || !s.store.Apply(snapshot) {
		return
	}

	s.logger.Info("runtime snapshot updated",
		"target", snapshot.Spec.Target,
		"gateway", snapshot.Spec.Gateway,
		"version", snapshot.Spec.Version,
	)
	s.ads.NotifySnapshotsChanged()
}

func (s *Server) deleteSnapshotObject(obj any) {
	snapshot, ok := objectAs[*resource.RuntimeSnapshot](obj)
	if !ok || !s.store.Delete(snapshot) {
		return
	}

	s.logger.Info("runtime snapshot removed",
		"target", snapshot.Spec.Target,
		"gateway", snapshot.Spec.Gateway,
	)
	s.ads.NotifySnapshotsChanged()
}

func objectAs[T any](obj any) (T, bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	value, ok := obj.(T)
	return value, ok
}
