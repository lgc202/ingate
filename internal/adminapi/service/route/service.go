package route

import (
	"context"

	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 Route 查询用例
type Service struct {
	store *routestore.Store
}

// New 创建 Route service
func New(store *routestore.Store) *Service {
	return &Service{store: store}
}

// List 查询 Route 列表
func (s *Service) List(ctx context.Context) (*resource.RouteList, error) {
	return s.store.List(ctx)
}

// Get 查询单个 Route
func (s *Service) Get(ctx context.Context, name string) (*resource.Route, error) {
	return s.store.Get(ctx, name)
}
