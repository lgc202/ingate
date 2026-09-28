package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Route 将写入交给 API Server，将查询交给 MySQL 投影。
type Route struct {
	client gatewayclient.RouteInterface
	store  *mysql.Store
}

// NewRoute 创建 Route Repository。
func NewRoute(client gatewayclient.RouteInterface, store *mysql.Store) *Route {
	return &Route{client: client, store: store}
}

// Create 创建 Route，并返回 API Server 保存后的业务状态。
func (r *Route) Create(ctx context.Context, route domain.Route) (domain.Route, error) {
	created, err := r.client.Create(ctx, routeToAPI(route), metav1.CreateOptions{})
	if err != nil {
		return domain.Route{}, translateResourceError(err)
	}

	return routeFromAPI(created), nil
}

// Get 从 MySQL 查询模型读取指定 Route。
func (r *Route) Get(ctx context.Context, name string) (domain.Route, error) {
	route, err := r.store.GetRoute(ctx, name)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return domain.Route{}, fmt.Errorf("%w: %w", ErrNotFound, err)
		}
		return domain.Route{}, err
	}

	return routeFromMySQL(route), nil
}

// List 从 MySQL 查询模型按页码读取 Route。
func (r *Route) List(ctx context.Context, size, number int32) ([]domain.Route, int64, error) {
	result, err := r.store.ListRoutes(ctx, size, number)
	if err != nil {
		return nil, 0, err
	}

	routes := make([]domain.Route, len(result.Items))
	for i, route := range result.Items {
		routes[i] = routeFromMySQL(route)
	}

	return routes, result.TotalCount, nil
}

// Update 使用 API Server 当前对象核对版本，并替换 Route 的可编辑配置。
func (r *Route) Update(ctx context.Context, route domain.Route) (domain.Route, error) {
	current, err := r.client.Get(ctx, route.Name, metav1.GetOptions{})
	if err != nil {
		return domain.Route{}, translateResourceError(err)
	}

	if current.ResourceVersion != route.Version {
		return domain.Route{}, ErrConflict
	}

	current.Spec = routeToAPI(route).Spec
	updated, err := r.client.Update(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		return domain.Route{}, translateResourceError(err)
	}

	return routeFromAPI(updated), nil
}

// Delete 删除指定 Route。
func (r *Route) Delete(ctx context.Context, name string) error {
	return translateResourceError(r.client.Delete(ctx, name, metav1.DeleteOptions{}))
}

func routeToAPI(route domain.Route) *gatewayv1alpha1.Route {
	return &gatewayv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: route.Name},
		Spec: gatewayv1alpha1.RouteSpec{
			GatewayName:  route.GatewayName,
			ListenerName: route.ListenerName,
			Path: gatewayv1alpha1.PathMatch{
				Type:  gatewayv1alpha1.PathMatchType(route.Path.Type),
				Value: route.Path.Value,
			},
			UpstreamName: route.UpstreamName,
		},
	}
}

func routeFromAPI(route *gatewayv1alpha1.Route) domain.Route {
	return domain.Route{
		Name:         route.Name,
		GatewayName:  route.Spec.GatewayName,
		ListenerName: route.Spec.ListenerName,
		Path: domain.PathMatch{
			Type:  string(route.Spec.Path.Type),
			Value: route.Spec.Path.Value,
		},
		UpstreamName: route.Spec.UpstreamName,
		CreateTime:   route.CreationTimestamp.Time,
		Version:      route.ResourceVersion,
	}
}

func routeFromMySQL(route mysql.Route) domain.Route {
	return domain.Route{
		Name:         route.Name,
		GatewayName:  route.GatewayName,
		ListenerName: route.ListenerName,
		Path: domain.PathMatch{
			Type:  route.PathType,
			Value: route.PathValue,
		},
		UpstreamName: route.UpstreamName,
		CreateTime:   route.CreatedAt,
		Version:      route.ResourceVersion,
	}
}
