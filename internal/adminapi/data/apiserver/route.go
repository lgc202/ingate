package apiserver

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// RouteStore 读写 Route 声明式资源。
type RouteStore struct {
	client clientset.Interface
}

// NewRouteStore 创建 Route Store。
func NewRouteStore(client clientset.Interface) *RouteStore {
	return &RouteStore{client: client}
}

// ListPage 分页返回 Route。
func (s *RouteStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.Route], error) {
	routes, err := s.client.GatewayV1().Routes().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.Route]{}, listError("routes", err)
	}
	return biz.PageResult[resource.Route]{Items: routes.Items, NextCursor: routes.Continue}, nil
}

// Get 返回指定 Route。
func (s *RouteStore) Get(ctx context.Context, routeID string) (*resource.Route, error) {
	route, err := s.client.GatewayV1().Routes().Get(ctx, routeID, metav1.GetOptions{})
	return route, resourceError("get", "route", routeID, err)
}

// Create 创建 Route。
func (s *RouteStore) Create(
	ctx context.Context,
	routeID string,
	spec resource.RouteSpec,
) (*resource.Route, error) {
	route := &resource.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindRoute),
		},
		ObjectMeta: metav1.ObjectMeta{Name: routeID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().Routes().Create(ctx, route, metav1.CreateOptions{})
	return created, resourceError("create", "route", routeID, err)
}

// ReplaceSpec 完整替换 Route 配置。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *RouteStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.Route,
	spec resource.RouteSpec,
) (*resource.Route, error) {
	routeID := observed.Name
	updated, err := updateResource(
		ctx,
		s.client.GatewayV1().Routes(),
		observed,
		func(route *resource.Route) { route.Spec = spec },
	)
	if apierrors.IsConflict(err) {
		return nil, fmt.Errorf("replace route %q after conflict retries: %w", routeID, err)
	}
	return updated, resourceError("replace", "route", routeID, err)
}

// Delete 删除 Route。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *RouteStore) Delete(
	ctx context.Context,
	observed *resource.Route,
) error {
	routeID := observed.Name
	err := deleteResource(ctx, s.client.GatewayV1().Routes(), observed)
	if apierrors.IsConflict(err) {
		return fmt.Errorf("delete route %q after conflict retries: %w", routeID, err)
	}
	return resourceError("delete", "route", routeID, err)
}
