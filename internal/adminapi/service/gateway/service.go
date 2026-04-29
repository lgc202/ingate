package gateway

import (
	"context"
	"slices"

	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	runtimestore "github.com/lgc202/ingate/internal/adminapi/store/runtime"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 Gateway 查询用例
type Service struct {
	store    *gatewaystore.Store
	routes   *routestore.Store
	runtime  *runtimestore.Store
	upstream *upstreamstore.Store
}

// New 创建 Gateway service
func New(store *gatewaystore.Store, routes *routestore.Store, runtime *runtimestore.Store, upstream *upstreamstore.Store) *Service {
	return &Service{store: store, routes: routes, runtime: runtime, upstream: upstream}
}

// List 查询 Gateway 列表
func (s *Service) List(ctx context.Context) (*resource.GatewayList, error) {
	return s.store.List(ctx)
}

// Get 查询单个 Gateway
func (s *Service) Get(ctx context.Context, name string) (*resource.Gateway, error) {
	return s.store.Get(ctx, name)
}

// Overview 查询 Gateway 详情页聚合视图
func (s *Service) Overview(ctx context.Context, name string) (*Overview, error) {
	gateway, err := s.store.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	routeList, err := s.routes.List(ctx)
	if err != nil {
		return nil, err
	}
	upstreamList, err := s.upstream.List(ctx)
	if err != nil {
		return nil, err
	}
	snapshotList, err := s.runtime.List(ctx)
	if err != nil {
		return nil, err
	}

	routes := make([]resource.Route, 0)
	upstreamNames := map[string]struct{}{}
	for _, route := range routeList.Items {
		if !slices.Contains(route.Spec.ParentRefs, name) {
			continue
		}
		routes = append(routes, route)
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.UpstreamRefs {
				upstreamNames[ref.Name] = struct{}{}
			}
		}
	}

	upstreams := make([]resource.Upstream, 0, len(upstreamNames))
	for _, upstream := range upstreamList.Items {
		if _, ok := upstreamNames[upstream.Name]; ok {
			upstreams = append(upstreams, upstream)
		}
	}

	snapshots := make([]resource.RuntimeSnapshot, 0)
	for _, snapshot := range snapshotList.Items {
		if snapshot.Spec.Gateway == name {
			snapshots = append(snapshots, snapshot)
		}
	}

	return &Overview{
		Gateway:          gateway,
		Routes:           routes,
		Upstreams:        upstreams,
		RuntimeSnapshots: snapshots,
	}, nil
}
