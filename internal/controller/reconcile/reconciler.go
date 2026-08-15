// Package reconcile 将完整声明式资源集合收敛为一个 Envoy 配置域
package reconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"k8s.io/client-go/util/workqueue"

	"github.com/lgc202/ingate/internal/controller/compiler"
	"github.com/lgc202/ingate/internal/controller/delivery"
	controllerstatus "github.com/lgc202/ingate/internal/controller/status"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

type queueKey string

const (
	queueKeyDesiredConfig    queueKey = "desired-config"
	queueKeyProgrammedStatus queueKey = "programmed-status"
)

// Reconciler 编排一个 Ingate 配置域的全量编译、发布和资源状态收敛
type Reconciler struct {
	resources    *resourceCache
	delivery     *delivery.Delivery
	statusWriter *controllerstatus.Writer
	queue        workqueue.TypedRateLimitingInterface[queueKey]
	logger       *slog.Logger
	started      chan struct{}
	done         chan struct{}
	cancel       context.CancelFunc
}

// New 创建使用固定全局 key 收敛整个配置域的 Reconciler
func New(
	client clientset.Interface,
	resyncPeriod time.Duration,
	configDelivery *delivery.Delivery,
	logger *slog.Logger,
) (*Reconciler, error) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[queueKey](),
		workqueue.TypedRateLimitingQueueConfig[queueKey]{Name: "resource-reconcile"},
	)
	resources, err := newResourceCache(client, resyncPeriod, func() {
		queue.Add(queueKeyDesiredConfig)
	})
	if err != nil {
		queue.ShutDown()
		return nil, err
	}
	return &Reconciler{
		resources:    resources,
		delivery:     configDelivery,
		statusWriter: controllerstatus.NewWriter(client.GatewayV1()),
		queue:        queue,
		logger:       logger,
		started:      make(chan struct{}),
		done:         make(chan struct{}),
	}, nil
}

// Start 同步 informer cache 后执行唯一的全配置域收敛循环
func (r *Reconciler) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	close(r.started)
	defer close(r.done)

	stopQueue := context.AfterFunc(runCtx, r.queue.ShutDown)
	deliveryWatcherDone := make(chan struct{})
	go func() {
		defer close(deliveryWatcherDone)
		for {
			select {
			case <-runCtx.Done():
				return
			case <-r.delivery.Changes():
				r.queue.Add(queueKeyProgrammedStatus)
			}
		}
	}()
	defer func() {
		cancel()
		<-deliveryWatcherDone
		stopQueue()
		r.queue.ShutDown()
		r.resources.shutdown()
	}()

	if err := r.resources.start(runCtx); err != nil {
		return err
	}
	if runCtx.Err() != nil {
		return nil
	}

	r.queue.Add(queueKeyDesiredConfig)
	for r.processNextWorkItem(runCtx) {
	}
	return nil
}

// Stop 停止资源监听与收敛循环，并等待 informer 和状态监听协程退出
func (r *Reconciler) Stop(ctx context.Context) error {
	select {
	case <-r.started:
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

	r.cancel()

	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Reconciler) processNextWorkItem(ctx context.Context) bool {
	key, shutdown := r.queue.Get()
	if shutdown {
		return false
	}
	defer r.queue.Done(key)

	var err error
	switch key {
	case queueKeyDesiredConfig:
		err = r.reconcileDesiredConfig(ctx)
	case queueKeyProgrammedStatus:
		err = r.reconcileProgrammedStatus(ctx)
	default:
		err = fmt.Errorf("unknown reconcile queue key %q", key)
	}
	if err != nil {
		if ctx.Err() != nil {
			r.queue.Forget(key)
			return true
		}
		r.logger.Error("reconcile work item failed", "queue_key", key, "err", err)
		r.queue.AddRateLimited(key)
		return true
	}
	r.queue.Forget(key)
	return true
}

func (r *Reconciler) reconcileDesiredConfig(ctx context.Context) error {
	resources, err := r.resources.list()
	if err != nil {
		return err
	}

	result := compiler.Compile(resources)

	var deliveryErr error
	if result.HasErrors() {
		if err := r.delivery.CancelCandidate(ctx); err != nil {
			deliveryErr = fmt.Errorf("cancel pending Envoy configuration after compile errors: %w", err)
		}
	} else if err := r.delivery.Submit(ctx, result); err != nil {
		deliveryErr = fmt.Errorf("submit Envoy configuration %q: %w", result.Version, err)
	}

	statusErr := r.statusWriter.ApplyCompileResult(ctx, resources, result.Diagnostics, r.delivery.Status())
	if statusErr != nil {
		statusErr = fmt.Errorf("apply resource compile status: %w", statusErr)
	}
	return errors.Join(deliveryErr, statusErr)
}

func (r *Reconciler) reconcileProgrammedStatus(ctx context.Context) error {
	resources, err := r.resources.list()
	if err != nil {
		return err
	}
	if err := r.statusWriter.ApplyProgrammed(ctx, resources, r.delivery.Status()); err != nil {
		return fmt.Errorf("apply resource programmed status: %w", err)
	}
	return nil
}
