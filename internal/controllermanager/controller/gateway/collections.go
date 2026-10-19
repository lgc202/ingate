package gateway

import (
	"context"
	"fmt"

	"istio.io/istio/pkg/kube/krt"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// compilationSource 统一拥有六类源和跨资源派生集合，依赖全部创建后再等待初始同步。
// 所有集合受同一个 Manager Context 管理，不另建 Informer 或后台发布循环。
type compilationSource struct {
	gateways         *resourceSource[*gatewayv1alpha1.Gateway, gatewayv1alpha1.Gateway]
	httpRoutes       *resourceSource[*gatewayv1alpha1.HTTPRoute, httpRoute]
	backends         *resourceSource[*gatewayv1alpha1.Backend, networkBackend]
	aiBackends       *resourceSource[*gatewayv1alpha1.AIBackend, modelBackend]
	modelRoutes      *resourceSource[*gatewayv1alpha1.ModelRoute, modelRoute]
	mcpRoutes        *resourceSource[*gatewayv1alpha1.MCPRoute, mcpRoute]
	bindings         krt.Collection[*modelBackendBinding]
	boundSync        krt.HandlerRegistration
	selection        krt.Singleton[gatewaySelection]
	entries          krt.Collection[*listenerEntry]
	entriesByGateway krt.Index[client.ObjectKey, *listenerEntry]
	httpAttachments  krt.Collection[*routeAttachment]
	modelAttachments krt.Collection[*routeAttachment]
	mcpAttachments   krt.Collection[*routeAttachment]
	routeIndexes     entryRouteIndexes
	tables           krt.Collection[*entryTable]
	httpStatuses     krt.Collection[*httpRouteStatus]
	entrySync        []krt.HandlerRegistration
}

var _ source.SyncingSource = (*compilationSource)(nil)

func newCompilationSource(resourceCache cache.Cache) *compilationSource {
	return &compilationSource{
		gateways:    newResourceSource(resourceCache, &gatewayv1alpha1.Gateway{}, projectGateway),
		httpRoutes:  newResourceSource(resourceCache, &gatewayv1alpha1.HTTPRoute{}, compileHTTPRoute),
		backends:    newResourceSource(resourceCache, &gatewayv1alpha1.Backend{}, compileNetworkBackend),
		aiBackends:  newResourceSource(resourceCache, &gatewayv1alpha1.AIBackend{}, compileModelBackend),
		modelRoutes: newResourceSource(resourceCache, &gatewayv1alpha1.ModelRoute{}, compileModelRoute),
		mcpRoutes:   newResourceSource(resourceCache, &gatewayv1alpha1.MCPRoute{}, compileMCPRoute),
	}
}

func (s *compilationSource) Start(
	ctx context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	for _, resource := range s.sources() {
		if err := resource.Start(ctx, queue); err != nil {
			return err
		}
	}

	s.registerBindings(ctx, queue)
	s.registerEntryGraph(ctx, queue)

	return nil
}

func (s *compilationSource) WaitForSync(ctx context.Context) error {
	for _, resource := range s.sources() {
		if err := resource.WaitForSync(ctx); err != nil {
			return err
		}
	}

	if !s.boundSync.WaitUntilSynced(ctx.Done()) {
		return fmt.Errorf("wait for model backend bindings: %w", ctx.Err())
	}

	return s.waitForEntries(ctx)
}

func (s *compilationSource) sources() []source.SyncingSource {
	return []source.SyncingSource{s.gateways, s.httpRoutes, s.backends, s.aiBackends, s.modelRoutes, s.mcpRoutes}
}

func (s *compilationSource) registerBindings(
	ctx context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	bind := func(ctx krt.HandlerContext, backend *modelBackend) *modelBackendBinding {
		return bindModelBackend(ctx, backend, s.backends.projected)
	}
	s.bindings = krt.NewPointerCollection(s.aiBackends.projected, bind, krt.WithStop(ctx.Done()), krt.WithName("gateway/model-backend-bindings"))
	s.boundSync = s.bindings.RegisterBatch(func(changes []krt.Event[*modelBackendBinding]) {
		for range changes {
			queue.Add(sharedRequest())
		}
	}, true)
}
