package runtime

import (
	"context"

	runtimestore "github.com/lgc202/ingate/internal/adminapi/store/runtime"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 RuntimeSnapshot 查询用例
type Service struct {
	store *runtimestore.Store
}

// New 创建 RuntimeSnapshot service
func New(store *runtimestore.Store) *Service {
	return &Service{store: store}
}

// List 查询 RuntimeSnapshot 列表
func (s *Service) List(ctx context.Context) (*resource.RuntimeSnapshotList, error) {
	return s.store.List(ctx)
}

// Get 查询单个 RuntimeSnapshot
func (s *Service) Get(ctx context.Context, name string) (*resource.RuntimeSnapshot, error) {
	return s.store.Get(ctx, name)
}
