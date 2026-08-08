package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// RouteRepository 读写 Route 声明式资源
type RouteRepository struct {
	client clientset.Interface
}

// NewRoute 创建 Route Repository
func NewRoute(client clientset.Interface) *RouteRepository {
	return &RouteRepository{client: client}
}

// List 查询 Route 列表
func (r *RouteRepository) List(ctx context.Context) (*resource.RouteList, error) {
	return r.client.GatewayV1().Routes().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 Route
func (r *RouteRepository) Get(ctx context.Context, name string) (*resource.Route, error) {
	return r.client.GatewayV1().Routes().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 Route
func (r *RouteRepository) Create(ctx context.Context, route *resource.Route) (*resource.Route, error) {
	return r.client.GatewayV1().Routes().Create(ctx, route, metav1.CreateOptions{})
}

// Update 更新 Route
func (r *RouteRepository) Update(ctx context.Context, route *resource.Route) (*resource.Route, error) {
	return r.client.GatewayV1().Routes().Update(ctx, route, metav1.UpdateOptions{})
}

// Delete 删除 Route
func (r *RouteRepository) Delete(ctx context.Context, name string) error {
	return r.client.GatewayV1().Routes().Delete(ctx, name, metav1.DeleteOptions{})
}
