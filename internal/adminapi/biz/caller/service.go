// Package caller 处理 Caller、Route 授权和访问密钥生命周期
package caller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Repository 定义 Caller 管理需要的持久化能力
type Repository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Caller], error)
	Get(ctx context.Context, callerID string) (*resource.Caller, error)
	Create(ctx context.Context, callerID string, spec resource.CallerSpec) (*resource.Caller, error)
	Update(ctx context.Context, callerID string, generation int64, spec resource.CallerSpec) (*resource.Caller, error)
	Delete(ctx context.Context, callerID string, generation int64) error
}

// RouteRepository 定义 Caller 授权 Route 时需要的查询能力
type RouteRepository interface {
	Get(ctx context.Context, routeID string) (*resource.Route, error)
}

// CreateInput 描述创建 Caller 并签发首个访问密钥所需的信息
type CreateInput struct {
	Spec             resource.CallerSpec
	AccessKeyName    string
	AccessKeyExpires *time.Time
}

// Service 协调 Caller 权限、访问密钥和持久化
type Service struct {
	repository Repository
	routes     RouteRepository
}

// NewService 创建 Caller 业务服务
func NewService(repository Repository, routes RouteRepository) *Service {
	return &Service{repository: repository, routes: routes}
}

// List 查询 Caller 列表
func (s *Service) List(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Caller], error) {
	return s.repository.ListPage(ctx, page)
}

// Get 查询单个 Caller
func (s *Service) Get(ctx context.Context, callerID string) (*resource.Caller, error) {
	return s.repository.Get(ctx, callerID)
}

// Create 创建 Caller 并签发首个访问密钥
func (s *Service) Create(ctx context.Context, input CreateInput) (*resource.Caller, IssuedKey, error) {
	if err := s.ensureDisplayNameAvailable(ctx, "", input.Spec.DisplayName); err != nil {
		return nil, IssuedKey{}, err
	}
	if err := s.validateAuthorizedRoutes(ctx, input.Spec.RouteRefs); err != nil {
		return nil, IssuedKey{}, err
	}
	issued, err := issueAccessKey(input.AccessKeyName, input.AccessKeyExpires)
	if err != nil {
		return nil, IssuedKey{}, err
	}
	input.Spec.AccessKeys = []resource.AccessKey{issued.AccessKey}
	caller, err := s.repository.Create(ctx, uuid.NewString(), input.Spec)
	if err != nil {
		return nil, IssuedKey{}, err
	}
	return caller, issued, nil
}

// Update 更新 Caller 名称、启用状态和 Route 权限，不改写现有密钥
func (s *Service) Update(
	ctx context.Context,
	callerID string,
	version int64,
	spec resource.CallerSpec,
) (*resource.Caller, error) {
	current, err := s.repository.Get(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, callerVersionConflict(current)
	}
	if spec.DisplayName != current.Spec.DisplayName {
		if err := s.ensureDisplayNameAvailable(ctx, callerID, spec.DisplayName); err != nil {
			return nil, err
		}
	}
	if err := s.validateAuthorizedRoutes(ctx, spec.RouteRefs); err != nil {
		return nil, err
	}
	spec.AccessKeys = current.Spec.AccessKeys
	return s.updateCaller(ctx, current, spec)
}

// Delete 删除 Caller；历史请求仍使用 Caller ID 保留归属
func (s *Service) Delete(ctx context.Context, callerID string, version int64) error {
	current, err := s.repository.Get(ctx, callerID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return callerVersionConflict(current)
	}
	if err := s.repository.Delete(ctx, callerID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return callerVersionConflict(current)
		}
		return err
	}
	return nil
}

func (s *Service) updateCaller(ctx context.Context, current *resource.Caller, spec resource.CallerSpec) (*resource.Caller, error) {
	updated, err := s.repository.Update(ctx, current.Name, current.Generation, spec)
	if errors.Is(err, biz.ErrResourceVersionConflict) {
		return nil, callerVersionConflict(current)
	}
	return updated, err
}

func callerVersionConflict(caller *resource.Caller) error {
	return biz.NewVersionConflict(
		caller.Name,
		fmt.Sprintf("调用方 %q 已被其他用户修改，请刷新后重试", caller.Spec.DisplayName),
	)
}
