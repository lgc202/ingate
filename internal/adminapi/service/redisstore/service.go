package redisstore

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/adminapi/store/ratelimitpolicy"
	redisstorestore "github.com/lgc202/ingate/internal/adminapi/store/redisstore"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 RedisStore 管理用例
type Service struct {
	store             *redisstorestore.Store
	rateLimitPolicies *ratelimitpolicystore.Store
}

// New 创建 RedisStore service
func New(store *redisstorestore.Store, rateLimitPolicies *ratelimitpolicystore.Store) *Service {
	return &Service{store: store, rateLimitPolicies: rateLimitPolicies}
}

// List 查询 RedisStore 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	stores, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListResult{RedisStores: stores.Items}, nil
}

// Get 查询单个 RedisStore
func (s *Service) Get(ctx context.Context, redisStoreID string) (*RedisStoreResult, error) {
	store, err := s.store.Get(ctx, redisStoreID)
	if err != nil {
		return nil, err
	}
	return &RedisStoreResult{RedisStore: store}, nil
}

// Create 创建 RedisStore
func (s *Service) Create(ctx context.Context, params CreateRedisStoreParams) (string, error) {
	if err := s.validateNameUnique(ctx, params.Name, ""); err != nil {
		return "", err
	}

	created, err := s.store.Create(ctx, redisStoreResource(uuid.NewString(), "", params.RedisStoreParams))
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 RedisStore
func (s *Service) Update(ctx context.Context, redisStoreID string, params UpdateRedisStoreParams) error {
	current, err := s.store.Get(ctx, redisStoreID)
	if err != nil {
		return err
	}
	if err := validateVersion(resource.ResourceRedisStores, redisStoreID, params.Version, current.ResourceVersion); err != nil {
		return err
	}
	if err := s.validateNameUnique(ctx, params.Name, redisStoreID); err != nil {
		return err
	}

	next := current.DeepCopy()
	next.Spec = redisStoreResource(next.Name, next.ResourceVersion, params.RedisStoreParams).Spec
	_, err = s.store.Update(ctx, next)
	return err
}

// Delete 删除 RedisStore，仍被全局限流策略引用时拒绝删除
func (s *Service) Delete(ctx context.Context, redisStoreID string) error {
	policies, err := s.rateLimitPolicies.List(ctx)
	if err != nil {
		return err
	}
	for _, policy := range policies.Items {
		if policy.Spec.Global != nil && policy.Spec.Global.RedisRef == redisStoreID {
			return xerrors.NewUserError(fmt.Sprintf("Redis 配置 %q 仍被限流策略 %q 引用", redisStoreID, policy.Spec.DisplayName))
		}
	}
	return s.store.Delete(ctx, redisStoreID)
}

func (s *Service) validateNameUnique(ctx context.Context, name, excludeID string) error {
	stores, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range stores.Items {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return xerrors.NewUserError(fmt.Sprintf("Redis 配置名称 %q 已存在", name))
		}
	}
	return nil
}

func redisStoreResource(id, version string, params RedisStoreParams) *resource.RedisStore {
	return &resource.RedisStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindRedisStore),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            id,
			ResourceVersion: version,
		},
		Spec: resource.RedisStoreSpec{
			DisplayName:          params.Name,
			Description:          params.Description,
			Mode:                 params.Mode,
			Address:              params.Address,
			DB:                   params.DB,
			TLS:                  params.TLS,
			Username:             params.Username,
			PasswordRef:          params.PasswordRef,
			ConnectTimeoutMillis: params.ConnectTimeoutMillis,
			CommandTimeoutMillis: params.CommandTimeoutMillis,
		},
	}
}

func validateVersion(resourceName resource.ResourceName, name, submittedVersion, currentVersion string) error {
	if submittedVersion == "" {
		return xerrors.NewUserError("资源版本不能为空")
	}
	if submittedVersion == currentVersion {
		return nil
	}
	return xerrors.NewUserError(fmt.Sprintf("%s %q 已被更新，请刷新后重试", resourceName, name))
}
