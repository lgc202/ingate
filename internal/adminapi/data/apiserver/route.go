package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// RouteRepository 读写 Route 声明式资源
type RouteRepository struct {
	client clientset.Interface
}

// NewRouteRepository 创建 Route Repository
func NewRouteRepository(client clientset.Interface) *RouteRepository {
	return &RouteRepository{client: client}
}

// ListPage 分页查询 Route 列表
func (r *RouteRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Route], error) {
	routes, err := r.client.GatewayV1().Routes().List(ctx, pageOptions(page))
	if err != nil {
		return biz.PageResult[resource.Route]{}, pageError("routes", err)
	}
	return biz.PageResult[resource.Route]{Items: routes.Items, NextCursor: routes.Continue}, nil
}

// Get 查询单个 Route
func (r *RouteRepository) Get(ctx context.Context, name string) (*resource.Route, error) {
	route, err := r.client.GatewayV1().Routes().Get(ctx, name, metav1.GetOptions{})
	return route, resourceError("get", "route", name, err)
}

// Create 创建 Route
func (r *RouteRepository) Create(ctx context.Context, id string, spec resource.RouteSpec) (*resource.Route, error) {
	route := &resource.Route{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindRoute)},
		ObjectMeta: metav1.ObjectMeta{Name: id},
		Spec:       spec,
	}
	created, err := r.client.GatewayV1().Routes().Create(ctx, route, metav1.CreateOptions{})
	return created, resourceError("create", "route", id, err)
}

// Update 更新 Route，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *RouteRepository) Update(
	ctx context.Context,
	id string,
	generation int64,
	spec resource.RouteSpec,
) (*resource.Route, error) {
	var updated *resource.Route
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Routes().Get(ctx, id, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		updated, err = r.client.GatewayV1().Routes().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return updated, resourceError("update", "route", id, err)
}

// Delete 删除 Route
func (r *RouteRepository) Delete(ctx context.Context, id string, generation int64) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Routes().Get(ctx, id, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		resourceVersion := current.ResourceVersion
		return r.client.GatewayV1().Routes().Delete(ctx, id, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &resourceVersion},
		})
	})
	return resourceError("delete", "route", id, err)
}
