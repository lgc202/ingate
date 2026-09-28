package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository"
)

const defaultPageSize = 50

var (
	// ErrInvalidArgument 表示网关参数不合法。
	ErrInvalidArgument = errors.New("invalid gateway argument")
	// ErrAlreadyExists 表示同名网关已存在。
	ErrAlreadyExists = errors.New("gateway already exists")
	// ErrNotFound 表示指定网关不存在。
	ErrNotFound = errors.New("gateway not found")
	// ErrConflict 表示网关版本已发生变化。
	ErrConflict = errors.New("gateway version conflict")
)

// CreateParams 保存创建 Gateway 所需的参数。
type CreateParams struct {
	Name        string
	DisplayName string
	Listeners   []domain.Listener
}

// UpdateParams 保存完整替换 Gateway 可编辑配置所需的参数。
type UpdateParams struct {
	Name        string
	Version     string
	DisplayName string
	Listeners   []domain.Listener
}

// ListResult 保存一页网关和本次查询的总数。
type ListResult struct {
	Items      []domain.Gateway
	TotalCount int64
}

// Service 实现 Gateway 管理用例。
type Service struct {
	gateways *repository.Gateway
}

// New 创建 Gateway Service。
func New(gateways *repository.Gateway) *Service {
	return &Service{gateways: gateways}
}

// Create 创建 Gateway。
func (s *Service) Create(
	ctx context.Context,
	params CreateParams,
) (domain.Gateway, error) {
	gateway := domain.Gateway{
		Name:        params.Name,
		DisplayName: params.DisplayName,
		Listeners:   params.Listeners,
	}

	created, err := s.gateways.Create(ctx, gateway)
	if err != nil {
		return domain.Gateway{}, translateRepositoryError(err)
	}

	return created, nil
}

// Get 读取指定 Gateway。
func (s *Service) Get(ctx context.Context, name string) (domain.Gateway, error) {
	gateway, err := s.gateways.Get(ctx, name)
	if err != nil {
		return domain.Gateway{}, translateRepositoryError(err)
	}

	return gateway, nil
}

// List 分页列出 Gateway。
func (s *Service) List(ctx context.Context, pageSize, pageNum int32) (ListResult, error) {
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageNum == 0 {
		pageNum = 1
	}

	gateways, totalCount, err := s.gateways.List(ctx, pageSize, pageNum)
	if err != nil {
		return ListResult{}, translateRepositoryError(err)
	}

	return ListResult{Items: gateways, TotalCount: totalCount}, nil
}

// Update 完整替换 Gateway 的可编辑配置，并检查调用方读取的版本。
func (s *Service) Update(ctx context.Context, params UpdateParams) (domain.Gateway, error) {
	gateway := domain.Gateway{
		Name:        params.Name,
		Version:     params.Version,
		DisplayName: params.DisplayName,
		Listeners:   params.Listeners,
	}

	updated, err := s.gateways.Update(ctx, gateway)
	if err != nil {
		return domain.Gateway{}, translateRepositoryError(err)
	}

	return updated, nil
}

// Delete 删除指定 Gateway。
func (s *Service) Delete(ctx context.Context, name string) error {
	return translateRepositoryError(s.gateways.Delete(ctx, name))
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
