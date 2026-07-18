// Package reconcile 将完整声明式资源集合收敛为一个 Envoy 配置域
package reconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"k8s.io/client-go/util/workqueue"

	controllerstatus "github.com/lgc202/ingate/internal/controller/status"
	"github.com/lgc202/ingate/internal/envoy/config"
	"github.com/lgc202/ingate/internal/envoy/delivery"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/pkg/generated/informers/externalversions"
)

type queueKey string

const (
	queueKeyConfig queueKey = "config"
	queueKeyStatus queueKey = "status"
)

// Reconciler 监听一个 Ingate 配置域内的全部资源并执行原子全量编译
type Reconciler struct {
	factory   informers.SharedInformerFactory
	resources resourceListers
	compiler  config.Compiler
	delivery  *delivery.Delivery
	statuses  *controllerstatus.Writer
	queue     workqueue.TypedRateLimitingInterface[queueKey]
	logger    *slog.Logger
}

// New 创建只使用唯一全局 queue key 的配置域 Reconciler
func New(
	client clientset.Interface,
	resyncPeriod time.Duration,
	configDelivery *delivery.Delivery,
	logger *slog.Logger,
) (*Reconciler, error) {
	factory := informers.NewSharedInformerFactory(client, resyncPeriod)
	gatewayInformers := factory.Gateway().V1()
	gatewayInformer := gatewayInformers.Gateways()
	certificateInformer := gatewayInformers.Certificates()
	routeInformer := gatewayInformers.Routes()
	upstreamInformer := gatewayInformers.Upstreams()
	rateLimitPolicyInformer := gatewayInformers.RateLimitPolicies()
	accessControlPolicyInformer := gatewayInformers.AccessControlPolicies()

	r := &Reconciler{
		factory: factory,
		resources: resourceListers{
			gateways:              gatewayInformer.Lister(),
			certificates:          certificateInformer.Lister(),
			routes:                routeInformer.Lister(),
			upstreams:             upstreamInformer.Lister(),
			rateLimitPolicies:     rateLimitPolicyInformer.Lister(),
			accessControlPolicies: accessControlPolicyInformer.Lister(),
		},
		delivery: configDelivery,
		statuses: controllerstatus.NewWriter(client),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[queueKey](),
			workqueue.TypedRateLimitingQueueConfig[queueKey]{Name: "controller"},
		),
		logger: logger,
	}
	if err := r.registerEventHandlers([]eventRegistration{
		{name: "Gateway", informer: gatewayInformer.Informer()},
		{name: "Certificate", informer: certificateInformer.Informer()},
		{name: "Route", informer: routeInformer.Informer()},
		{name: "Upstream", informer: upstreamInformer.Informer()},
		{name: "RateLimitPolicy", informer: rateLimitPolicyInformer.Informer()},
		{name: "AccessControlPolicy", informer: accessControlPolicyInformer.Informer()},
	}); err != nil {
		r.queue.ShutDown()
		return nil, err
	}
	return r, nil
}

// Run 同步 informer cache 后执行唯一的全配置域收敛循环
func (r *Reconciler) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	stopQueue := context.AfterFunc(runCtx, r.queue.ShutDown)
	deliveryWatcherDone := make(chan struct{})
	go func() {
		defer close(deliveryWatcherDone)
		for {
			select {
			case <-runCtx.Done():
				return
			case <-r.delivery.Changes():
				r.queue.Add(queueKeyStatus)
			}
		}
	}()
	defer func() {
		cancel()
		<-deliveryWatcherDone
		stopQueue()
		r.queue.ShutDown()
		r.factory.Shutdown()
	}()

	r.factory.Start(runCtx.Done())
	for resourceType, synced := range r.factory.WaitForCacheSync(runCtx.Done()) {
		if synced {
			continue
		}
		if runCtx.Err() != nil {
			return nil
		}
		return fmt.Errorf("sync informer cache for %v", resourceType)
	}

	r.queue.Add(queueKeyConfig)
	for r.processNextWorkItem(runCtx) {
	}
	return nil
}

func (r *Reconciler) processNextWorkItem(ctx context.Context) bool {
	key, shutdown := r.queue.Get()
	if shutdown {
		return false
	}
	defer r.queue.Done(key)

	var err error
	switch key {
	case queueKeyConfig:
		err = r.reconcileConfig(ctx)
	case queueKeyStatus:
		err = r.reconcileStatus(ctx)
	default:
		err = fmt.Errorf("unknown reconcile queue key %q", key)
	}
	if err != nil {
		if ctx.Err() != nil {
			r.queue.Forget(key)
			return true
		}
		r.logger.Error("reconcile controller state", "key", key, "error", err)
		r.queue.AddRateLimited(key)
		return true
	}
	r.queue.Forget(key)
	return true
}

func (r *Reconciler) reconcileConfig(ctx context.Context) error {
	resources, err := r.resources.build()
	if err != nil {
		return err
	}

	result := r.compiler.Compile(resources)

	var deliveryErr error
	if result.HasErrors() {
		if err := r.delivery.CancelCandidate(ctx); err != nil {
			deliveryErr = fmt.Errorf("cancel pending Envoy configuration after compile errors: %w", err)
		}
	} else if err := r.delivery.Submit(ctx, result); err != nil {
		deliveryErr = fmt.Errorf("submit Envoy configuration %q: %w", result.Version, err)
	}

	statusErr := r.statuses.ApplyCompileResult(ctx, resources, result.Diagnostics, r.delivery.Status())
	if statusErr != nil {
		statusErr = fmt.Errorf("apply resource compile status: %w", statusErr)
	}
	return errors.Join(deliveryErr, statusErr)
}

func (r *Reconciler) reconcileStatus(ctx context.Context) error {
	resources, err := r.resources.build()
	if err != nil {
		return err
	}
	if err := r.statuses.ApplyProgrammed(ctx, resources, r.delivery.Status()); err != nil {
		return fmt.Errorf("apply resource programmed status: %w", err)
	}
	return nil
}
