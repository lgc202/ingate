package route

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository"
)

const defaultPageSize = 50

var (
	// ErrInvalidArgument 表示路由参数不合法。
	ErrInvalidArgument = errors.New("invalid route argument")
	// ErrAlreadyExists 表示同名路由已存在。
	ErrAlreadyExists = errors.New("route already exists")
	// ErrNotFound 表示指定路由不存在。
	ErrNotFound = errors.New("route not found")
	// ErrConflict 表示路由版本已发生变化。
	ErrConflict = errors.New("route version conflict")
)

// CreateParams 保存创建 Route 所需的参数。
type CreateParams struct {
	Name         string
	GatewayName  string
	ListenerName string
	Path         domain.PathMatch
	UpstreamName string
}

// UpdateParams 保存完整替换 Route 可编辑配置所需的参数。
type UpdateParams struct {
	Name         string
	Version      string
	GatewayName  string
	ListenerName string
	Path         domain.PathMatch
	UpstreamName string
}

// ListResult 保存一页路由和本次查询的总数。
type ListResult struct {
	Items      []domain.Route
	TotalCount int64
}

// Service 实现 Route 管理用例。
type Service struct {
	routes *repository.Route
}

// New 创建 Route Service。
func New(routes *repository.Route) *Service {
	return &Service{routes: routes}
}

// Create 创建 Route。
func (s *Service) Create(ctx context.Context, params CreateParams) (domain.Route, error) {
	route := domain.Route{
		Name:         params.Name,
		GatewayName:  params.GatewayName,
		ListenerName: params.ListenerName,
		Path:         params.Path,
		UpstreamName: params.UpstreamName,
	}

	created, err := s.routes.Create(ctx, route)
	if err != nil {
		return domain.Route{}, translateRepositoryError(err)
	}

	return created, nil
}

// Get 读取指定 Route。
func (s *Service) Get(ctx context.Context, name string) (domain.Route, error) {
	route, err := s.routes.Get(ctx, name)
	if err != nil {
		return domain.Route{}, translateRepositoryError(err)
	}

	return route, nil
}

// List 分页列出 Route。
func (s *Service) List(ctx context.Context, pageSize, pageNum int32) (ListResult, error) {
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageNum == 0 {
		pageNum = 1
	}

	routes, totalCount, err := s.routes.List(ctx, pageSize, pageNum)
	if err != nil {
		return ListResult{}, translateRepositoryError(err)
	}

	return ListResult{Items: routes, TotalCount: totalCount}, nil
}

// Update 完整替换 Route 的可编辑配置，并检查调用方读取的版本。
func (s *Service) Update(ctx context.Context, params UpdateParams) (domain.Route, error) {
	route := domain.Route{
		Name:         params.Name,
		Version:      params.Version,
		GatewayName:  params.GatewayName,
		ListenerName: params.ListenerName,
		Path:         params.Path,
		UpstreamName: params.UpstreamName,
	}

	updated, err := s.routes.Update(ctx, route)
	if err != nil {
		return domain.Route{}, translateRepositoryError(err)
	}

	return updated, nil
}

// Delete 删除指定 Route。
func (s *Service) Delete(ctx context.Context, name string) error {
	return translateRepositoryError(s.routes.Delete(ctx, name))
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
