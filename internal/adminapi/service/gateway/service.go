package gateway

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	runtimestore "github.com/lgc202/ingate/internal/adminapi/store/runtime"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
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
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	gateways, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	routes, err := s.routes.List(ctx)
	if err != nil {
		return nil, err
	}
	upstreams, err := s.upstream.List(ctx)
	if err != nil {
		return nil, err
	}
	snapshots, err := s.runtime.List(ctx)
	if err != nil {
		return nil, err
	}

	return &ListResult{
		Gateways:         gateways.Items,
		Routes:           routes.Items,
		Upstreams:        upstreams.Items,
		RuntimeSnapshots: snapshots.Items,
	}, nil
}

// Get 查询单个 Gateway
func (s *Service) Get(ctx context.Context, name string) (*GatewayResult, error) {
	gateway, err := s.store.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return s.gatewayResult(ctx, gateway)
}

// Create 创建 Gateway
func (s *Service) Create(ctx context.Context, gateway *resource.Gateway) error {
	_, err := s.store.Create(ctx, gateway)
	return err
}

// Update 更新 Gateway
func (s *Service) Update(ctx context.Context, name string, gateway *resource.Gateway) error {
	if gateway.Name != name {
		return apierrors.NewBadRequest("gateway name cannot be changed")
	}

	current, err := s.store.Get(ctx, name)
	if err != nil {
		return err
	}
	if err := validateVersion(resource.ResourceGateways, name, gateway.ResourceVersion, current.ResourceVersion); err != nil {
		return err
	}
	next := current.DeepCopy()
	applyGatewayUpdate(next, gateway)
	_, err = s.store.Update(ctx, next)
	return err
}

// SetEnabled 更新 Gateway 启停状态
func (s *Service) SetEnabled(ctx context.Context, name string, enabled bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.store.Get(ctx, name)
		if err != nil {
			return err
		}

		next := current.DeepCopy()
		if next.Annotations == nil {
			next.Annotations = map[string]string{}
		}
		next.Annotations[resource.AnnotationGatewayEnabled] = strconv.FormatBool(enabled)

		_, err = s.store.Update(ctx, next)
		return err
	})
}

// Delete 删除 Gateway，仍有关联路由时拒绝删除
func (s *Service) Delete(ctx context.Context, name string) error {
	routes, err := s.routes.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes.Items {
		if slices.Contains(route.Spec.ParentRefs, name) {
			return apierrors.NewBadRequest(fmt.Sprintf("gateway %q still has attached routes", name))
		}
	}
	return s.store.Delete(ctx, name)
}

// Overview 查询 Gateway 详情页聚合数据
func (s *Service) Overview(ctx context.Context, name string) (*DetailResult, error) {
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

	return &DetailResult{
		Gateway:          gateway,
		Routes:           routes,
		Upstreams:        upstreams,
		RuntimeSnapshots: snapshots,
	}, nil
}

func applyGatewayUpdate(next *resource.Gateway, submitted *resource.Gateway) {
	next.Spec = submitted.Spec
	if next.Annotations == nil {
		next.Annotations = map[string]string{}
	}
	for _, key := range []string{
		resource.AnnotationGatewayDescription,
		resource.AnnotationGatewayEnabled,
		resource.AnnotationGatewayHostnames,
	} {
		delete(next.Annotations, key)
	}
	for key, value := range submitted.Annotations {
		if key == resource.AnnotationGatewayDescription ||
			key == resource.AnnotationGatewayEnabled ||
			key == resource.AnnotationGatewayHostnames {
			next.Annotations[key] = value
		}
	}
}

func validateVersion(resourceName resource.ResourceName, name, submittedVersion, currentVersion string) error {
	if submittedVersion == "" || submittedVersion == currentVersion {
		return nil
	}
	return apierrors.NewConflict(
		resource.Resource(resourceName),
		name,
		fmt.Errorf("resource version changed, current version is %s", currentVersion),
	)
}

func (s *Service) gatewayResult(ctx context.Context, gateway *resource.Gateway) (*GatewayResult, error) {
	routes, err := s.routes.List(ctx)
	if err != nil {
		return nil, err
	}
	upstreams, err := s.upstream.List(ctx)
	if err != nil {
		return nil, err
	}
	snapshots, err := s.runtime.List(ctx)
	if err != nil {
		return nil, err
	}

	return &GatewayResult{
		Gateway:          gateway,
		Routes:           routes.Items,
		Upstreams:        upstreams.Items,
		RuntimeSnapshots: snapshots.Items,
	}, nil
}
