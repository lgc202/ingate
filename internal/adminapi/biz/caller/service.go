// Package caller 处理 Caller、Route 授权和访问密钥生命周期
package caller

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/pkg/accesskey"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义 Caller 管理需要的持久化能力
type Repository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Caller], error)
	Get(context.Context, string) (*resource.Caller, error)
	Create(context.Context, string, resource.CallerSpec) (*resource.Caller, error)
	Update(context.Context, string, int64, resource.CallerSpec) (*resource.Caller, error)
	Delete(context.Context, string, int64) error
}

// RouteRepository 定义 Caller 授权 Route 时需要的查询能力
type RouteRepository interface {
	Get(context.Context, string) (*resource.Route, error)
}

// IssuedKey 包含只在本次调用中存在的完整访问密钥
type IssuedKey struct {
	AccessKey resource.AccessKey
	Secret    string
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
	if err := s.validateDisplayName(ctx, "", input.Spec.DisplayName); err != nil {
		return nil, IssuedKey{}, err
	}
	if err := s.validateRoutes(ctx, input.Spec.RouteRefs); err != nil {
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
		if err := s.validateDisplayName(ctx, callerID, spec.DisplayName); err != nil {
			return nil, err
		}
	}
	if err := s.validateRoutes(ctx, spec.RouteRefs); err != nil {
		return nil, err
	}
	spec.AccessKeys = current.Spec.AccessKeys
	return s.update(ctx, current, spec)
}

// IssueAccessKey 为现有 Caller 签发一份新的独立密钥
func (s *Service) IssueAccessKey(
	ctx context.Context,
	callerID string,
	version int64,
	name string,
	expiresAt *time.Time,
) (IssuedKey, error) {
	current, err := s.repository.Get(ctx, callerID)
	if err != nil {
		return IssuedKey{}, err
	}
	if version != current.Generation {
		return IssuedKey{}, callerVersionConflict(current)
	}
	for _, key := range current.Spec.AccessKeys {
		if key.DisplayName == name {
			return IssuedKey{}, biz.NewUserError(fmt.Sprintf("访问密钥名称 %q 已存在", name))
		}
	}
	issued, err := issueAccessKey(name, expiresAt)
	if err != nil {
		return IssuedKey{}, err
	}
	current.Spec.AccessKeys = append(current.Spec.AccessKeys, issued.AccessKey)
	if _, err := s.update(ctx, current, current.Spec); err != nil {
		return IssuedKey{}, err
	}
	return issued, nil
}

// DisableAccessKey 立即停用 Caller 下的一份访问密钥
func (s *Service) DisableAccessKey(
	ctx context.Context,
	callerID,
	accessKeyID string,
	version int64,
) (*resource.Caller, error) {
	current, err := s.repository.Get(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, callerVersionConflict(current)
	}
	index := slices.IndexFunc(current.Spec.AccessKeys, func(key resource.AccessKey) bool {
		return key.ID == accessKeyID
	})
	if index < 0 {
		return nil, biz.NewUserError("访问密钥不存在")
	}
	if !current.Spec.AccessKeys[index].Enabled {
		return current, nil
	}
	current.Spec.AccessKeys[index].Enabled = false
	return s.update(ctx, current, current.Spec)
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

func (s *Service) update(ctx context.Context, current *resource.Caller, spec resource.CallerSpec) (*resource.Caller, error) {
	updated, err := s.repository.Update(ctx, current.Name, current.Generation, spec)
	if errors.Is(err, biz.ErrResourceVersionConflict) {
		return nil, callerVersionConflict(current)
	}
	return updated, err
}

func (s *Service) validateDisplayName(ctx context.Context, callerID, displayName string) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(caller resource.Caller) (bool, error) {
		if caller.Name != callerID && caller.Spec.DisplayName == displayName {
			return true, biz.NewUserError(fmt.Sprintf("调用方名称 %q 已存在", displayName))
		}
		return false, nil
	})
}

func (s *Service) validateRoutes(ctx context.Context, routeIDs []string) error {
	seen := make(map[string]struct{}, len(routeIDs))
	for _, routeID := range routeIDs {
		if _, exists := seen[routeID]; exists {
			return biz.NewUserError("同一个路由只能授权一次")
		}
		seen[routeID] = struct{}{}
		route, err := s.routes.Get(ctx, routeID)
		if err != nil {
			return err
		}
		if route.Spec.AccessMode != resource.RouteAccessCaller {
			return biz.NewUserError(fmt.Sprintf("路由 %q 不使用调用方密钥", route.Spec.DisplayName))
		}
	}
	return nil
}

func issueAccessKey(name string, expiresAt *time.Time) (IssuedKey, error) {
	now := time.Now().UTC()
	if expiresAt != nil && !expiresAt.After(now) {
		return IssuedKey{}, biz.NewUserError("访问密钥到期时间必须晚于当前时间")
	}
	keyID := uuid.NewString()
	secret, err := accesskey.Generate(keyID)
	if err != nil {
		return IssuedKey{}, fmt.Errorf("generate access key: %w", err)
	}
	key := resource.AccessKey{
		ID:           keyID,
		DisplayName:  name,
		SecretDigest: accesskey.Digest(secret),
		Enabled:      true,
		CreatedAt:    metav1.NewTime(now),
	}
	if expiresAt != nil {
		value := metav1.NewTime(expiresAt.UTC())
		key.ExpiresAt = &value
	}
	return IssuedKey{AccessKey: key, Secret: secret}, nil
}

func callerVersionConflict(caller *resource.Caller) error {
	return biz.NewVersionConflictError(
		caller.Name,
		fmt.Sprintf("调用方 %q 已被其他用户修改，请刷新后重试", caller.Spec.DisplayName),
	)
}
