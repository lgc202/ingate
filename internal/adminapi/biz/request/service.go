package request

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound 表示请求记录不存在或已经超过明细保留期
	ErrNotFound = errors.New("request record not found")
	// ErrUnavailable 表示请求分析组件当前无法提供查询
	ErrUnavailable = errors.New("request analytics unavailable")
)

// Repository 定义请求记录查询需要的 Analytics 能力
type Repository interface {
	List(context.Context, ListOptions) (Page, error)
	Get(context.Context, string, time.Time) (*Record, error)
}

// Service 提供不依赖 Analytics gRPC 协议的请求记录查询
type Service struct {
	repository Repository
}

// NewService 创建请求记录业务服务
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// List 按时间倒序查询请求记录
func (s *Service) List(ctx context.Context, options ListOptions) (Page, error) {
	return s.repository.List(ctx, options)
}

// Get 查询单次请求记录
func (s *Service) Get(ctx context.Context, id string, startedAt time.Time) (*Record, error) {
	return s.repository.Get(ctx, id, startedAt)
}
