package app

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"k8s.io/client-go/tools/cache"

	resource "github.com/lgc202/ingate-next/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate-next/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate-next/pkg/generated/informers/externalversions"
)

// Server 维护 RuntimeSnapshot 观察状态，后续在此基础上提供 xDS 协议
type Server struct {
	factory   informers.SharedInformerFactory
	target    string
	snapshots map[string]*resource.RuntimeSnapshot
	stdout    io.Writer
	mu        sync.RWMutex
}

// NewServer 创建 xDS 配置观察服务
func NewServer(client clientset.Interface, target string, resyncPeriod time.Duration, stdout io.Writer) *Server {
	return &Server{
		factory:   informers.NewSharedInformerFactory(client, resyncPeriod),
		target:    target,
		snapshots: map[string]*resource.RuntimeSnapshot{},
		stdout:    stdout,
	}
}

// Run 启动 RuntimeSnapshot watch 主循环
func (s *Server) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		s.factory.Shutdown()
	}()

	if err := s.registerEventHandlers(); err != nil {
		return err
	}
	s.factory.Start(runCtx.Done())
	if err := s.waitForCacheSync(runCtx); err != nil {
		return err
	}

	fmt.Fprintf(s.stdout, "ingate-xds watching target=%s\n", s.target)
	<-runCtx.Done()
	return runCtx.Err()
}

func (s *Server) registerEventHandlers() error {
	_, err := s.factory.Gateway().V1().RuntimeSnapshots().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			s.applySnapshotObject(obj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			s.deleteSnapshotObject(oldObj)
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
	if !ok || snapshot.Spec.Target != s.target {
		return
	}

	s.mu.Lock()
	s.snapshots[snapshot.Spec.Gateway] = snapshot.DeepCopy()
	s.mu.Unlock()

	fmt.Fprintf(s.stdout, "snapshot updated target=%s gateway=%s version=%s\n", snapshot.Spec.Target, snapshot.Spec.Gateway, snapshot.Spec.Version)
}

func (s *Server) deleteSnapshotObject(obj any) {
	snapshot, ok := objectAs[*resource.RuntimeSnapshot](obj)
	if !ok || snapshot.Spec.Target != s.target {
		return
	}

	s.mu.Lock()
	delete(s.snapshots, snapshot.Spec.Gateway)
	s.mu.Unlock()

	fmt.Fprintf(s.stdout, "snapshot removed target=%s gateway=%s\n", snapshot.Spec.Target, snapshot.Spec.Gateway)
}

func objectAs[T any](obj any) (T, bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	value, ok := obj.(T)
	return value, ok
}
