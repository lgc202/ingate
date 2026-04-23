package resolvedgateway

import (
	"context"
	"errors"
	"net"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	controllerruntime "github.com/lgc202/ingate/internal/controlplane/controller/runtime"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	controllerstatus "github.com/lgc202/ingate/internal/controlplane/controller/status"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

type BundleLoader interface {
	Load(gatewayKey shared.ObjectKey) (*ResourceBundle, error)
}

type ResolvedGatewayPersister interface {
	Upsert(ctx context.Context, resolvedGateway *gatewayv1alpha1.ResolvedGateway) (*gatewayv1alpha1.ResolvedGateway, error)
}

type StatusWriter interface {
	MarkSuccess(
		ctx context.Context,
		gateway *gatewayv1alpha1.Gateway,
		routes []*gatewayv1alpha1.Route,
		backends []*gatewayv1alpha1.Backend,
		certificates []*gatewayv1alpha1.Certificate,
		authPolicies []*policyv1alpha1.AuthPolicy,
		trafficPolicies []*policyv1alpha1.TrafficPolicy,
		rg *gatewayv1alpha1.ResolvedGateway,
	) error
	MarkFailure(ctx context.Context, key shared.ObjectKey, err error) error
}

type Controller struct {
	client    clientset.Interface
	queue     shared.GatewayQueue
	loader    BundleLoader
	persister ResolvedGatewayPersister
	status    StatusWriter
}

func NewController(ctx *controllerruntime.Context) *Controller {
	if ctx == nil {
		return &Controller{}
	}
	return NewControllerWithDependencies(ctx.Clientset, ctx.GatewayQueue, NewLoader(ctx))
}

func NewControllerWithDependencies(client clientset.Interface, queue shared.GatewayQueue, loader BundleLoader) *Controller {
	return &Controller{
		client:    client,
		queue:     queue,
		loader:    loader,
		persister: NewPersister(client),
		status:    controllerstatus.NewUpdater(client),
	}
}

func (c *Controller) Run(ctx context.Context, workers int) {
	if c == nil || c.queue == nil || workers < 1 {
		<-ctx.Done()
		return
	}
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, func(ctx context.Context) { c.runWorker(ctx) }, time.Second)
	}
	<-ctx.Done()
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNext(ctx) {
	}
}

func (c *Controller) processNext(ctx context.Context) bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)

	if err := c.Reconcile(ctx, key); err != nil {
		if shouldRequeue(err) {
			c.queue.Requeue(key)
		} else {
			c.queue.Forget(key)
		}
		return true
	}

	c.queue.Forget(key)
	return true
}

func shouldRequeue(err error) bool {
	if err == nil {
		return false
	}
	if apierrors.IsNotFound(err) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	return apierrors.IsConflict(err) ||
		apierrors.IsTimeout(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsServiceUnavailable(err) ||
		apierrors.IsInternalError(err) ||
		errors.Is(err, context.DeadlineExceeded)
}
