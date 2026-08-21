// Package biz 编排声明式资源到 Envoy 配置的控制面用例
package biz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"k8s.io/client-go/util/workqueue"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	"github.com/lgc202/ingate/internal/controller/biz/delivery"
)

type queueKey string

const (
	queueKeyDesiredConfig    queueKey = "desired-config"
	queueKeyProgrammedStatus queueKey = "programmed-status"
)

// ResourceWatcher 向控制循环提供完整资源事实和变更通知
//
// 接口定义在消费方，避免 biz 依赖 informer 和生成客户端
type ResourceWatcher interface {
	Start(context.Context) error
	Stop()
	List() (compiler.Resources, error)
	Changes() <-chan struct{}
}

// StatusWriter 将编译和发布结果回写为声明式资源状态
type StatusWriter interface {
	ApplyCompileResult(context.Context, compiler.Resources, []compiler.Diagnostic, delivery.Status) error
	ApplyProgrammed(context.Context, compiler.Resources, delivery.Status) error
}

// Controller 将一个 Ingate 配置域持续收敛为可被 Envoy 接受的配置
type Controller struct {
	resources    ResourceWatcher
	delivery     *delivery.Delivery
	statusWriter StatusWriter
	queue        workqueue.TypedRateLimitingInterface[queueKey]
	logger       *slog.Logger
	started      chan struct{}
	done         chan struct{}
	cancel       context.CancelFunc
}

// NewController 创建使用固定全局 key 收敛整个配置域的控制循环
func NewController(
	resources ResourceWatcher,
	statusWriter StatusWriter,
	configDelivery *delivery.Delivery,
	logger *slog.Logger,
) *Controller {
	return &Controller{
		resources:    resources,
		delivery:     configDelivery,
		statusWriter: statusWriter,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[queueKey](),
			workqueue.TypedRateLimitingQueueConfig[queueKey]{Name: "configuration-reconcile"},
		),
		logger:  logger,
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start 同步资源缓存后执行唯一的全配置域收敛循环
func (c *Controller) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	c.cancel = cancel
	close(c.started)
	defer close(c.done)
	defer c.resources.Stop()

	if err := c.resources.Start(runCtx); err != nil {
		return err
	}
	if runCtx.Err() != nil {
		return nil
	}

	changesDone := make(chan struct{})
	go func() {
		defer close(changesDone)
		c.watchChanges(runCtx)
	}()
	defer func() {
		cancel()
		<-changesDone
	}()

	c.queue.Add(queueKeyDesiredConfig)
	for c.processNextWorkItem(runCtx) {
	}
	return nil
}

// Stop 停止资源监听与收敛循环，并等待内部协程退出
func (c *Controller) Stop(ctx context.Context) error {
	select {
	case <-c.started:
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

	c.cancel()

	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Controller) watchChanges(ctx context.Context) {
	// workqueue.Get 不接收 context，监听协程退出时负责关闭队列并唤醒控制循环
	defer c.queue.ShutDown()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.resources.Changes():
			c.queue.Add(queueKeyDesiredConfig)
		case <-c.delivery.Changes():
			c.queue.Add(queueKeyProgrammedStatus)
		}
	}
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)

	var err error
	switch key {
	case queueKeyDesiredConfig:
		err = c.reconcileDesiredConfig(ctx)
	case queueKeyProgrammedStatus:
		err = c.reconcileProgrammedStatus(ctx)
	default:
		err = fmt.Errorf("unknown reconcile queue key %q", key)
	}
	if err != nil {
		if ctx.Err() != nil {
			c.queue.Forget(key)
			return false
		}
		c.logger.Error("reconcile work item failed", "queue_key", key, "error", err)
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *Controller) reconcileDesiredConfig(ctx context.Context) error {
	resources, err := c.resources.List()
	if err != nil {
		return err
	}

	result := compiler.Compile(resources)

	var deliveryErr error
	if result.HasErrors() {
		if err := c.delivery.CancelCandidate(ctx); err != nil {
			deliveryErr = fmt.Errorf("cancel pending Envoy configuration after compile errors: %w", err)
		}
	} else if err := c.delivery.Submit(ctx, result); err != nil {
		deliveryErr = fmt.Errorf("submit Envoy configuration %q: %w", result.Version, err)
	}

	statusErr := c.statusWriter.ApplyCompileResult(ctx, resources, result.Diagnostics, c.delivery.Status())
	if statusErr != nil {
		statusErr = fmt.Errorf("apply resource compile status: %w", statusErr)
	}
	return errors.Join(deliveryErr, statusErr)
}

func (c *Controller) reconcileProgrammedStatus(ctx context.Context) error {
	resources, err := c.resources.List()
	if err != nil {
		return err
	}
	if err := c.statusWriter.ApplyProgrammed(ctx, resources, c.delivery.Status()); err != nil {
		return fmt.Errorf("apply resource programmed status: %w", err)
	}
	return nil
}
