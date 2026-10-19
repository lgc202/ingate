package gateway

import (
	"context"
	"fmt"
	"path"
	"slices"

	"istio.io/istio/pkg/kube/krt"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type httpRouteStatus struct {
	route   *httpRoute
	parents []httpRouteParentResolution
	entries map[string]*listenerEntry
}

type entryRouteIndexes struct {
	http  krt.Index[string, *routeAttachment]
	model krt.Index[string, *routeAttachment]
	mcp   krt.Index[string, *routeAttachment]
}

// ResourceName 让状态计算沿用 HTTPRoute 身份；状态不进入入口路由表。
func (s *httpRouteStatus) ResourceName() string {
	return s.route.ResourceName()
}

func (s *compilationSource) registerEntryGraph(
	ctx context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	stop := krt.WithStop(ctx.Done())
	s.selection = krt.NewSingleton(func(ctx krt.HandlerContext) *gatewaySelection {
		return selectGatewayEntries(krt.Fetch(ctx, s.gateways.projected))
	}, stop, krt.WithName("gateway/selection"))

	s.entries = krt.NewManyCollection(s.selection.AsCollection(), func(_ krt.HandlerContext, selection gatewaySelection) []*listenerEntry {
		var entries []*listenerEntry
		for _, gateway := range selection.accepted {
			for i, listener := range gateway.Spec.Listeners {
				entries = append(entries, &listenerEntry{origin: projectForwardingMeta(gateway), listener: listener, index: i})
			}
		}

		return entries
	}, stop, krt.WithName("gateway/entries"))
	s.entriesByGateway = krt.NewIndex(s.entries, "gateway", func(entry *listenerEntry) []client.ObjectKey {
		return []client.ObjectKey{{Namespace: entry.origin.Namespace, Name: entry.origin.Name}}
	})

	httpParents := krt.NewPointerCollection(s.httpRoutes.projected, func(_ krt.HandlerContext, route *httpRoute) *routeParents {
		return &routeParents{key: client.ObjectKey{Namespace: route.Namespace, Name: route.Name}, parents: route.parents}
	}, stop, krt.WithName("gateway/http-parents"))
	modelParents := krt.NewPointerCollection(s.modelRoutes.projected, func(_ krt.HandlerContext, route *modelRoute) *routeParents {
		return &routeParents{
			key:     client.ObjectKey{Namespace: route.Namespace, Name: route.Name},
			parents: route.parents, deleting: !route.DeletionTimestamp.IsZero(),
		}
	}, stop, krt.WithName("gateway/model-parents"))
	mcpParents := krt.NewPointerCollection(s.mcpRoutes.projected, func(_ krt.HandlerContext, route *mcpRoute) *routeParents {
		return &routeParents{
			key:     client.ObjectKey{Namespace: route.Namespace, Name: route.Name},
			parents: route.parents, deleting: !route.DeletionTimestamp.IsZero(),
		}
	}, stop, krt.WithName("gateway/mcp-parents"))

	s.httpAttachments = krt.NewPointerCollection(httpParents, s.attachRoute, stop, krt.WithName("gateway/http-attachments"))
	s.modelAttachments = krt.NewPointerCollection(modelParents, s.attachRoute, stop, krt.WithName("gateway/model-attachments"))
	s.mcpAttachments = krt.NewPointerCollection(mcpParents, s.attachRoute, stop, krt.WithName("gateway/mcp-attachments"))

	keys := func(route *routeAttachment) []string { return route.entries }
	s.routeIndexes = entryRouteIndexes{
		http:  krt.NewIndex(s.httpAttachments, "entry", keys),
		model: krt.NewIndex(s.modelAttachments, "entry", keys),
		mcp:   krt.NewIndex(s.mcpAttachments, "entry", keys),
	}

	s.tables = krt.NewPointerCollection(s.entries, s.compileEntry, stop, krt.WithName("gateway/entry-tables"))
	s.httpStatuses = krt.NewPointerCollection(s.httpRoutes.projected, s.compileHTTPStatus, stop, krt.WithName("gateway/http-statuses"))

	s.entrySync = []krt.HandlerRegistration{
		watchCompilation(s.selection.AsCollection(), queue),
		watchCompilation(s.tables, queue),
		watchCompilation(s.httpStatuses, queue),
	}
}

func (s *compilationSource) attachRoute(ctx krt.HandlerContext, route *routeParents) *routeAttachment {
	attached := &routeAttachment{route: route}
	if route.deleting {
		return attached
	}

	used := make(map[string]bool)
	for _, parent := range route.parents {
		key := client.ObjectKey{Namespace: route.key.Namespace, Name: parent.Name}
		for _, entry := range krt.Fetch(ctx, s.entries, krt.FilterIndex(s.entriesByGateway, key)) {
			if !entry.matches(route.key.Namespace, []gatewayv1alpha1.ParentReference{parent}) {
				continue
			}

			name := entry.ResourceName()
			if !used[name] {
				attached.entries = append(attached.entries, name)
				used[name] = true
			}
		}
	}

	slices.Sort(attached.entries)

	return attached
}

// compileEntry 的所有跨集合读取均经 Fetch，krt 因此只重算受依赖变化影响的入口。
func (s *compilationSource) compileEntry(ctx krt.HandlerContext, entry *listenerEntry) *entryTable {
	var snapshot resources
	httpFilter := krt.FilterIndex(s.routeIndexes.http, entry.ResourceName())
	for _, attached := range krt.Fetch(ctx, s.httpAttachments, httpFilter) {
		if route := krt.FetchOne(ctx, s.httpRoutes.projected, krt.FilterKey(attached.ResourceName())); route != nil {
			snapshot.httpRoutes = append(snapshot.httpRoutes, *route)
		}
	}

	modelFilter := krt.FilterIndex(s.routeIndexes.model, entry.ResourceName())
	for _, attached := range krt.Fetch(ctx, s.modelAttachments, modelFilter) {
		if route := krt.FetchOne(ctx, s.modelRoutes.projected, krt.FilterKey(attached.ResourceName())); route != nil {
			snapshot.modelRoutes = append(snapshot.modelRoutes, *route)
		}
	}

	mcpFilter := krt.FilterIndex(s.routeIndexes.mcp, entry.ResourceName())
	for _, attached := range krt.Fetch(ctx, s.mcpAttachments, mcpFilter) {
		if route := krt.FetchOne(ctx, s.mcpRoutes.projected, krt.FilterKey(attached.ResourceName())); route != nil {
			snapshot.mcpRoutes = append(snapshot.mcpRoutes, *route)
		}
	}

	snapshot = s.fetchEntryBackends(ctx, snapshot)

	return compileEntryTable(entry, snapshot)
}

func (s *compilationSource) compileHTTPStatus(ctx krt.HandlerContext, route *httpRoute) *httpRouteStatus {
	var gateways []gatewayv1alpha1.Gateway
	entries := make(map[string]*listenerEntry)
	for _, parent := range route.parents {
		key := client.ObjectKey{Namespace: route.Namespace, Name: parent.Name}
		for _, entry := range krt.Fetch(ctx, s.entries, krt.FilterIndex(s.entriesByGateway, key)) {
			if entries[entry.ResourceName()] == nil {
				entries[entry.ResourceName()] = entry
				gateways = append(gateways, entry.gateway())
			}
		}
	}

	// 同一 Gateway 的 Listener 必须先合并，不能让名称索引丢失兄弟入口。
	gateways = groupEntryGateways(gateways)
	snapshot := s.fetchEntryBackends(ctx, resources{httpRoutes: []*httpRoute{route}})
	result := resolution{}
	for i := range gateways {
		result.gateways = append(result.gateways, &gateways[i])
	}

	result.selectHTTPRoutes([]*httpRoute{route}, snapshot.backends)

	return &httpRouteStatus{route: route, parents: result.parents, entries: entries}
}

func (s *compilationSource) waitForEntries(ctx context.Context) error {
	for _, registration := range s.entrySync {
		if !registration.WaitUntilSynced(ctx.Done()) {
			return fmt.Errorf("wait for entry compilation: %w", ctx.Err())
		}
	}

	return nil
}

// fetchEntryBackends 只查询本入口引用的对象；缺失查询也由 krt 跟踪。
// 这里只使用网络身份与删除状态，端点变化不应触发路由表的重新组装。
func (s *compilationSource) fetchEntryBackends(ctx krt.HandlerContext, snapshot resources) resources {
	networks := make(map[client.ObjectKey]bool)
	for _, route := range snapshot.httpRoutes {
		for _, rule := range route.rules {
			for _, ref := range rule.refs {
				networks[client.ObjectKey{Namespace: route.Namespace, Name: ref.Name}] = true
			}
		}
	}

	protocols := make(map[client.ObjectKey]bool)
	for _, route := range snapshot.modelRoutes {
		for _, rule := range route.rules {
			for _, ref := range rule.refs {
				protocols[client.ObjectKey{Namespace: route.Namespace, Name: ref.Name}] = true
			}
		}
	}

	for key := range protocols {
		bindings := krt.PartialFetch(ctx, s.bindings,
			func(binding *modelBackendBinding) *modelBackendBinding { return binding },
			sameProtocolBinding, krt.FilterKey(path.Join(key.Namespace, key.Name)))
		for _, binding := range bindings {
			snapshot.aiBackends = append(snapshot.aiBackends, binding)
			networks[binding.backend.networkKey] = true
		}
	}

	for _, route := range snapshot.mcpRoutes {
		for _, ref := range route.refs {
			networks[client.ObjectKey{Namespace: route.Namespace, Name: ref.Name}] = true
		}
	}

	for key := range networks {
		backends := krt.PartialFetch(ctx, s.backends.projected,
			func(backend *networkBackend) *networkBackend { return backend },
			sameNetworkPresence, krt.FilterKey(path.Join(key.Namespace, key.Name)))
		snapshot.backends = append(snapshot.backends, backends...)
	}

	return snapshot
}

func sameProtocolBinding(a, b *modelBackendBinding) bool {
	return *a.backend == *b.backend && (a.network == nil) == (b.network == nil)
}

func sameNetworkPresence(a, b *networkBackend) bool {
	return a.key == b.key && a.deleting == b.deleting
}

func groupEntryGateways(gateways []gatewayv1alpha1.Gateway) []gatewayv1alpha1.Gateway {
	var grouped []gatewayv1alpha1.Gateway
	byKey := make(map[client.ObjectKey]int)
	for _, gateway := range gateways {
		key := client.ObjectKeyFromObject(&gateway)
		index, exists := byKey[key]
		if !exists {
			byKey[key] = len(grouped)
			grouped = append(grouped, gateway)
			continue
		}

		grouped[index].Spec.Listeners = append(grouped[index].Spec.Listeners, gateway.Spec.Listeners...)
	}

	return grouped
}

func watchCompilation[T any](collection krt.Collection[T], queue workqueue.TypedRateLimitingInterface[reconcile.Request]) krt.HandlerRegistration {
	return collection.RegisterBatch(func(changes []krt.Event[T]) {
		for range changes {
			queue.Add(sharedRequest())
		}
	}, true)
}
