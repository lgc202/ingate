// Package upstream 处理 Upstream 的管理规则和资源协作
package upstream

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Repository 定义 Upstream 管理需要的持久化能力
type Repository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Upstream], error)
	Get(ctx context.Context, upstreamID string) (*resource.Upstream, error)
	Create(ctx context.Context, upstreamID string, spec resource.UpstreamSpec) (*resource.Upstream, error)
	Update(ctx context.Context, upstreamID string, generation int64, spec resource.UpstreamSpec) (*resource.Upstream, error)
	Delete(ctx context.Context, upstreamID string, generation int64) error
}

// RouteRepository 定义删除 Upstream 时需要的 Route 查询能力
type RouteRepository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Route], error)
}

// Service 协调 Upstream 的校验、引用约束和持久化
type Service struct {
	repository Repository
	routes     RouteRepository
}

// UpdateInput 描述 Upstream 更新及敏感字段的保留语义
type UpdateInput struct {
	Version             int64
	Spec                resource.UpstreamSpec
	PreserveModelAPIKey bool
}

// NewService 创建 Upstream 业务服务
func NewService(repository Repository, routes RouteRepository) *Service {
	return &Service{repository: repository, routes: routes}
}

// List 查询 Upstream 列表
func (s *Service) List(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Upstream], error) {
	return s.repository.ListPage(ctx, page)
}

// Get 查询单个 Upstream
func (s *Service) Get(ctx context.Context, upstreamID string) (*resource.Upstream, error) {
	return s.repository.Get(ctx, upstreamID)
}

// Create 创建 Upstream
func (s *Service) Create(ctx context.Context, spec resource.UpstreamSpec) (*resource.Upstream, error) {
	if err := s.ensureDisplayNameAvailable(ctx, "", spec.DisplayName); err != nil {
		return nil, err
	}
	return s.repository.Create(ctx, uuid.NewString(), spec)
}

// Update 使用配置版本乐观更新 Upstream
func (s *Service) Update(
	ctx context.Context,
	upstreamID string,
	input UpdateInput,
) (*resource.Upstream, error) {
	current, err := s.repository.Get(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	if input.Version != current.Generation {
		return nil, upstreamVersionConflict(current)
	}
	if input.Spec.DisplayName != current.Spec.DisplayName {
		if err := s.ensureDisplayNameAvailable(ctx, upstreamID, input.Spec.DisplayName); err != nil {
			return nil, err
		}
	}
	if input.PreserveModelAPIKey && current.Spec.Model != nil && input.Spec.Model != nil {
		input.Spec.Model.APIKey = current.Spec.Model.APIKey
	}

	updated, err := s.repository.Update(ctx, upstreamID, current.Generation, input.Spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return nil, upstreamVersionConflict(current)
		}
		return nil, err
	}
	return updated, nil
}

// Delete 删除 Upstream，仍有关联路由时拒绝删除
func (s *Service) Delete(ctx context.Context, upstreamID string, version int64) error {
	current, err := s.repository.Get(ctx, upstreamID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return upstreamVersionConflict(current)
	}
	if err := s.ensureNotReferenced(ctx, current); err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, upstreamID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return upstreamVersionConflict(current)
		}
		return err
	}
	return nil
}

func (s *Service) ensureDisplayNameAvailable(ctx context.Context, upstreamID, displayName string) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(upstream resource.Upstream) (bool, error) {
		if upstream.Name != upstreamID && upstream.Spec.DisplayName == displayName {
			return true, biz.NewRuleViolation(fmt.Sprintf("服务名称 %q 已存在", displayName))
		}
		return false, nil
	})
}

func upstreamVersionConflict(upstream *resource.Upstream) error {
	return biz.NewVersionConflict(
		upstream.Name,
		fmt.Sprintf("服务 %q 已被其他用户修改，请刷新后重试", upstream.Spec.DisplayName),
	)
}
