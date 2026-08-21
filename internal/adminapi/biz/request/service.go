package request

import (
	"context"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

const (
	reasonRequestRecordNotFound = "REQUEST_RECORD_NOT_FOUND"
	reasonDependencyUnavailable = "DEPENDENCY_UNAVAILABLE"
)

var (
	// ErrNotFound 表示请求记录不存在或已经超过明细保留期
	ErrNotFound = kratoserrors.NotFound(reasonRequestRecordNotFound, "request record not found").
			WithMetadata(map[string]string{"user_message": "请求记录不存在或已超过明细保留期"})
	// ErrUnavailable 表示请求分析组件当前无法提供查询
	ErrUnavailable = kratoserrors.ServiceUnavailable(reasonDependencyUnavailable, "request analytics unavailable").
			WithMetadata(map[string]string{"user_message": "请求记录服务暂时不可用，请稍后重试"})
)

// Unavailable 保留 Analytics 返回的底层原因，同时向控制台暴露稳定错误语义
func Unavailable(cause error) error {
	return ErrUnavailable.WithCause(cause)
}

// Repository 定义请求记录查询需要的 Analytics 能力
type Repository interface {
	List(ctx context.Context, options ListOptions) (Page, error)
	Get(ctx context.Context, recordID string, startedAt time.Time) (*Record, error)
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
func (s *Service) Get(ctx context.Context, recordID string, startedAt time.Time) (*Record, error) {
	return s.repository.Get(ctx, recordID, startedAt)
}
