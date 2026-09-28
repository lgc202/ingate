package upstream

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository"
)

const defaultPageSize = 50

var (
	// ErrInvalidArgument 表示后端资源参数不合法。
	ErrInvalidArgument = errors.New("invalid upstream argument")
	// ErrAlreadyExists 表示同名后端资源已存在。
	ErrAlreadyExists = errors.New("upstream already exists")
	// ErrNotFound 表示指定后端资源不存在。
	ErrNotFound = errors.New("upstream not found")
	// ErrConflict 表示后端资源版本已发生变化。
	ErrConflict = errors.New("upstream version conflict")
)

// CreateParams 保存创建 Upstream 所需的参数。
type CreateParams struct {
	Name      string
	Endpoints []domain.Endpoint
}

// UpdateParams 保存完整替换 Upstream 可编辑配置所需的参数。
type UpdateParams struct {
	Name      string
	Version   string
	Endpoints []domain.Endpoint
}

// ListResult 保存一页后端资源和本次查询的总数。
type ListResult struct {
	Items      []domain.Upstream
	TotalCount int64
}

// Service 实现 Upstream 管理用例。
type Service struct {
	upstreams *repository.Upstream
}

// New 创建 Upstream Service。
func New(upstreams *repository.Upstream) *Service {
	return &Service{upstreams: upstreams}
}

// Create 创建 Upstream。
func (s *Service) Create(ctx context.Context, params CreateParams) (domain.Upstream, error) {
	upstream := domain.Upstream{
		Name:      params.Name,
		Endpoints: params.Endpoints,
	}

	created, err := s.upstreams.Create(ctx, upstream)
	if err != nil {
		return domain.Upstream{}, translateRepositoryError(err)
	}

	return created, nil
}

// Get 读取指定 Upstream。
func (s *Service) Get(ctx context.Context, name string) (domain.Upstream, error) {
	upstream, err := s.upstreams.Get(ctx, name)
	if err != nil {
		return domain.Upstream{}, translateRepositoryError(err)
	}

	return upstream, nil
}

// List 分页列出 Upstream。
func (s *Service) List(ctx context.Context, pageSize, pageNum int32) (ListResult, error) {
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageNum == 0 {
		pageNum = 1
	}

	upstreams, totalCount, err := s.upstreams.List(ctx, pageSize, pageNum)
	if err != nil {
		return ListResult{}, translateRepositoryError(err)
	}

	return ListResult{Items: upstreams, TotalCount: totalCount}, nil
}

// Update 完整替换 Upstream 的可编辑配置，并检查调用方读取的版本。
func (s *Service) Update(ctx context.Context, params UpdateParams) (domain.Upstream, error) {
	upstream := domain.Upstream{
		Name:      params.Name,
		Version:   params.Version,
		Endpoints: params.Endpoints,
	}

	updated, err := s.upstreams.Update(ctx, upstream)
	if err != nil {
		return domain.Upstream{}, translateRepositoryError(err)
	}

	return updated, nil
}

// Delete 删除指定 Upstream。
func (s *Service) Delete(ctx context.Context, name string) error {
	return translateRepositoryError(s.upstreams.Delete(ctx, name))
}

func translateRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrInvalid):
		return fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	case errors.Is(err, repository.ErrAlreadyExists):
		return fmt.Errorf("%w: %w", ErrAlreadyExists, err)
	case errors.Is(err, repository.ErrNotFound):
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case errors.Is(err, repository.ErrConflict):
		return fmt.Errorf("%w: %w", ErrConflict, err)
	default:
		return err
	}
}
