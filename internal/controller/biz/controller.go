// Package biz 编排声明式资源到 Envoy 配置的控制面用例
package biz

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"k8s.io/client-go/util/workqueue"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
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

// WasmModuleStore 隔离 Controller 与远端模块拉取、缓存和保留策略
type WasmModuleStore interface {
	Resolve(context.Context, *gatewayv1.WasmPlugin) (compiler.WasmModule, error)
	Retain([]compiler.ResourceGeneration)
}

// Controller 将一个 Ingate 配置域持续收敛为可被 Envoy 接受的配置
type Controller struct {
	resources    ResourceWatcher
	delivery     *delivery.Delivery
	statusWriter StatusWriter
	wasmModules  WasmModuleStore
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
	wasmModules WasmModuleStore,
	logger *slog.Logger,
) *Controller {
	return &Controller{
		resources:    resources,
		delivery:     configDelivery,
		statusWriter: statusWriter,
		wasmModules:  wasmModules,
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
	c.wasmModules.Retain(wasmModuleGenerations(resources.WasmPlugins, c.delivery.Status().ActiveResources))

	wasmModules, moduleDiagnostics := c.resolveWasmModules(ctx, resources.WasmPlugins)
	result := compiler.Compile(resources, wasmModules)
	deliveryResult := result
	result.Diagnostics = mergeWasmModuleDiagnostics(result.Diagnostics, moduleDiagnostics)

	var deliveryErr error
	// 仅安装但尚未被策略使用的插件校验失败不应冻结整个网关配置；依赖该插件的策略会在编译结果中产生阻塞诊断
	if deliveryResult.HasErrors() {
		if err := c.delivery.CancelCandidate(ctx); err != nil {
			deliveryErr = fmt.Errorf("cancel pending Envoy configuration after compile errors: %w", err)
		}
	} else if err := c.delivery.Submit(ctx, deliveryResult); err != nil {
		deliveryErr = fmt.Errorf("submit Envoy configuration %q: %w", result.Version, err)
	}

	statusErr := c.statusWriter.ApplyCompileResult(ctx, resources, result.Diagnostics, c.delivery.Status())
	if statusErr != nil {
		statusErr = fmt.Errorf("apply resource compile status: %w", statusErr)
	}
	return errors.Join(deliveryErr, statusErr)
}

func (c *Controller) resolveWasmModules(
	ctx context.Context,
	plugins []*gatewayv1.WasmPlugin,
) (map[string]compiler.WasmModule, []compiler.Diagnostic) {
	modules := make(map[string]compiler.WasmModule)
	var diagnostics []compiler.Diagnostic
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		module, err := c.wasmModules.Resolve(ctx, plugin)
		if err != nil {
			diagnostics = append(diagnostics, compiler.Diagnostic{
				Severity: compiler.SeverityError,
				Kind:     gatewayv1.KindWasmPlugin,
				ID:       plugin.Name,
				Reason:   compiler.ReasonArtifactUnavailable,
				Message:  fmt.Sprintf("prepare Wasm plugin %q module: %v", plugin.Name, err),
			})
			continue
		}
		modules[plugin.Name] = module
	}
	return modules, diagnostics
}

// mergeWasmModuleDiagnostics 用具体的拉取失败替换 compiler 对缺失模块的兜底诊断
//
// 即使模块拉取失败也必须完整编译其余资源，避免把尚未校验的资源误标为 Accepted
func mergeWasmModuleDiagnostics(
	compiled []compiler.Diagnostic,
	resolved []compiler.Diagnostic,
) []compiler.Diagnostic {
	failedPlugins := make(map[string]bool, len(resolved))
	for _, diagnostic := range resolved {
		failedPlugins[diagnostic.ID] = true
	}

	diagnostics := make([]compiler.Diagnostic, 0, len(compiled)+len(resolved))
	for _, diagnostic := range compiled {
		if diagnostic.Kind == gatewayv1.KindWasmPlugin &&
			diagnostic.Reason == compiler.ReasonCompileFailed &&
			failedPlugins[diagnostic.ID] {
			continue
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	diagnostics = append(diagnostics, resolved...)
	slices.SortFunc(diagnostics, func(a, b compiler.Diagnostic) int {
		return cmp.Or(
			cmp.Compare(a.Severity, b.Severity),
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.ID, b.ID),
			cmp.Compare(a.Reason, b.Reason),
			cmp.Compare(a.Message, b.Message),
		)
	})
	return diagnostics
}

func (c *Controller) reconcileProgrammedStatus(ctx context.Context) error {
	resources, err := c.resources.List()
	if err != nil {
		return err
	}
	deliveryStatus := c.delivery.Status()
	// Candidate ACK/NACK 后重新计算保护集合，使已退出 Active 的旧模块可以被容量淘汰
	c.wasmModules.Retain(wasmModuleGenerations(resources.WasmPlugins, deliveryStatus.ActiveResources))
	if err := c.statusWriter.ApplyProgrammed(ctx, resources, deliveryStatus); err != nil {
		return fmt.Errorf("apply resource programmed status: %w", err)
	}
	return nil
}

// wasmModuleGenerations 同时保留当前期望插件和 last-good xDS 仍引用的插件 generation
//
// Candidate 生效前不能只按最新声明式资源清理缓存，否则删除或升级插件会让 Active 配置的模块 URL 提前失效
func wasmModuleGenerations(
	plugins []*gatewayv1.WasmPlugin,
	active []compiler.ResourceGeneration,
) []compiler.ResourceGeneration {
	retained := make(map[compiler.ResourceGeneration]bool)
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		retained[compiler.ResourceGeneration{
			Kind:       gatewayv1.KindWasmPlugin,
			Name:       plugin.Name,
			UID:        plugin.UID,
			Generation: plugin.Generation,
		}] = true
	}
	for _, generation := range active {
		if generation.Kind == gatewayv1.KindWasmPlugin {
			retained[generation] = true
		}
	}
	result := make([]compiler.ResourceGeneration, 0, len(retained))
	for generation := range retained {
		result = append(result, generation)
	}
	slices.SortFunc(result, func(a, b compiler.ResourceGeneration) int {
		return cmp.Or(
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Name, b.Name),
			cmp.Compare(string(a.UID), string(b.UID)),
			cmp.Compare(a.Generation, b.Generation),
		)
	})
	return result
}
