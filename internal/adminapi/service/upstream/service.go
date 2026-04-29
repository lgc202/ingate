package upstream

import (
	"context"

	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 Upstream 查询用例
type Service struct {
	store *upstreamstore.Store
}

// New 创建 Upstream service
func New(store *upstreamstore.Store) *Service {
	return &Service{store: store}
}

// List 查询 Upstream 列表
func (s *Service) List(ctx context.Context) (*resource.UpstreamList, error) {
	return s.store.List(ctx)
}

// Get 查询单个 Upstream
func (s *Service) Get(ctx context.Context, name string) (*resource.Upstream, error) {
	return s.store.Get(ctx, name)
}
