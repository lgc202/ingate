package route

import (
	"context"
	"fmt"
	"strconv"

	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
)

// Service 承载 Route 查询用例
type Service struct {
	store    *routestore.Store
	gateways *gatewaystore.Store
	upstream *upstreamstore.Store
}

// New 创建 Route service
func New(store *routestore.Store, gateways *gatewaystore.Store, upstream *upstreamstore.Store) *Service {
	return &Service{store: store, gateways: gateways, upstream: upstream}
}

// List 查询 Route 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	routes, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListResult{Routes: routes.Items}, nil
}

// Get 查询单个 Route
func (s *Service) Get(ctx context.Context, name string) (*RouteResult, error) {
	route, err := s.store.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return &RouteResult{Route: route}, nil
}

// Create 创建 Route
func (s *Service) Create(ctx context.Context, route *resource.Route) error {
	if err := s.validateReferences(ctx, route); err != nil {
		return err
	}
	_, err := s.store.Create(ctx, route)
	return err
}

// Update 更新 Route
func (s *Service) Update(ctx context.Context, name string, route *resource.Route) error {
	if route.Name != name {
		return apierrors.NewBadRequest("route name cannot be changed")
	}
	if err := s.validateReferences(ctx, route); err != nil {
		return err
	}

	current, err := s.store.Get(ctx, name)
	if err != nil {
		return err
	}
	if err := validateVersion(resource.ResourceRoutes, name, route.ResourceVersion, current.ResourceVersion); err != nil {
		return err
	}
	next := current.DeepCopy()
	applyRouteUpdate(next, route)
	_, err = s.store.Update(ctx, next)
	return err
}

// SetEnabled 更新 Route 启停状态
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
		next.Annotations[resource.AnnotationRouteEnabled] = strconv.FormatBool(enabled)

		_, err = s.store.Update(ctx, next)
		return err
	})
}

// Delete 删除 Route
func (s *Service) Delete(ctx context.Context, name string) error {
	return s.store.Delete(ctx, name)
}

func (s *Service) validateReferences(ctx context.Context, route *resource.Route) error {
	for _, gatewayName := range route.Spec.ParentRefs {
		if _, err := s.gateways.Get(ctx, gatewayName); err != nil {
			return err
		}
	}
	for _, rule := range route.Spec.Rules {
		for _, ref := range rule.UpstreamRefs {
			if _, err := s.upstream.Get(ctx, ref.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyRouteUpdate(next *resource.Route, submitted *resource.Route) {
	next.Spec = submitted.Spec
	if next.Annotations == nil {
		next.Annotations = map[string]string{}
	}
	for _, key := range []string{
		resource.AnnotationRouteEnabled,
	} {
		delete(next.Annotations, key)
	}
	for key, value := range submitted.Annotations {
		if key == resource.AnnotationRouteEnabled {
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
