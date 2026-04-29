package gateway

import (
	"context"

	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 Gateway 查询用例
type Service struct {
	store *gatewaystore.Store
}

// New 创建 Gateway service
func New(store *gatewaystore.Store) *Service {
	return &Service{store: store}
}

// List 查询 Gateway 列表
func (s *Service) List(ctx context.Context) (*resource.GatewayList, error) {
	return s.store.List(ctx)
}

// Get 查询单个 Gateway
func (s *Service) Get(ctx context.Context, name string) (*resource.Gateway, error) {
	return s.store.Get(ctx, name)
}
