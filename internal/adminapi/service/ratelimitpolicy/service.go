package ratelimitpolicy

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	policybindingstore "github.com/lgc202/ingate/internal/adminapi/store/policybinding"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/adminapi/store/ratelimitpolicy"
	redisstorestore "github.com/lgc202/ingate/internal/adminapi/store/redisstore"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 RateLimitPolicy 管理用例
type Service struct {
	store          *ratelimitpolicystore.Store
	redisStores    *redisstorestore.Store
	policyBindings *policybindingstore.Store
}

// New 创建 RateLimitPolicy service
func New(store *ratelimitpolicystore.Store, redisStores *redisstorestore.Store, policyBindings *policybindingstore.Store) *Service {
	return &Service{store: store, redisStores: redisStores, policyBindings: policyBindings}
}

// List 查询 RateLimitPolicy 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	policies, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListResult{Policies: policies.Items}, nil
}

// Get 查询单个 RateLimitPolicy
func (s *Service) Get(ctx context.Context, policyID string) (*PolicyResult, error) {
	policy, err := s.store.Get(ctx, policyID)
	if err != nil {
		return nil, err
	}
	return &PolicyResult{Policy: policy}, nil
}

// Create 创建 RateLimitPolicy
func (s *Service) Create(ctx context.Context, params CreatePolicyParams) (string, error) {
	if err := s.validateNameUnique(ctx, params.Name, ""); err != nil {
		return "", err
	}
	if err := s.validateRedisRef(ctx, params.PolicyParams); err != nil {
		return "", err
	}

	created, err := s.store.Create(ctx, policyResource(uuid.NewString(), "", params.PolicyParams))
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 RateLimitPolicy
func (s *Service) Update(ctx context.Context, policyID string, params UpdatePolicyParams) error {
	current, err := s.store.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if err := validateVersion(resource.ResourceRateLimitPolicies, policyID, params.Version, current.ResourceVersion); err != nil {
		return err
	}
	if err := s.validateNameUnique(ctx, params.Name, policyID); err != nil {
		return err
	}
	if err := s.validateRedisRef(ctx, params.PolicyParams); err != nil {
		return err
	}

	next := current.DeepCopy()
	next.Spec = policyResource(next.Name, next.ResourceVersion, params.PolicyParams).Spec
	_, err = s.store.Update(ctx, next)
	return err
}

// SetEnabled 设置 RateLimitPolicy 启用状态
func (s *Service) SetEnabled(ctx context.Context, policyID string, enabled bool) error {
	current, err := s.store.Get(ctx, policyID)
	if err != nil {
		return err
	}
	next := current.DeepCopy()
	next.Spec.Enabled = enabled
	_, err = s.store.Update(ctx, next)
	return err
}

// Delete 删除 RateLimitPolicy，仍被 PolicyBinding 引用时拒绝删除
func (s *Service) Delete(ctx context.Context, policyID string) error {
	bindings, err := s.policyBindings.List(ctx)
	if err != nil {
		return err
	}
	for _, binding := range bindings.Items {
		for _, policy := range binding.Spec.Policies {
			if policy.Kind == resource.KindRateLimitPolicy && policy.Name == policyID {
				return xerrors.NewUserError(fmt.Sprintf("限流策略 %q 仍被策略绑定 %q 引用", policyID, binding.Spec.DisplayName))
			}
		}
	}
	return s.store.Delete(ctx, policyID)
}

func (s *Service) validateNameUnique(ctx context.Context, name, excludeID string) error {
	policies, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range policies.Items {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return xerrors.NewUserError(fmt.Sprintf("限流策略名称 %q 已存在", name))
		}
	}
	return nil
}

func (s *Service) validateRedisRef(ctx context.Context, params PolicyParams) error {
	if params.Mode != resource.RateLimitModeGlobal || params.Global == nil {
		return nil
	}
	if _, err := s.redisStores.Get(ctx, params.Global.RedisRef); err != nil {
		if apierrors.IsNotFound(err) {
			return xerrors.NewUserError(fmt.Sprintf("Redis 配置 %q 不存在", params.Global.RedisRef))
		}
		return err
	}
	return nil
}

func policyResource(id, version string, params PolicyParams) *resource.RateLimitPolicy {
	return &resource.RateLimitPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindRateLimitPolicy),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            id,
			ResourceVersion: version,
		},
		Spec: resource.RateLimitPolicySpec{
			DisplayName:   params.Name,
			Description:   params.Description,
			Enabled:       params.Enabled,
			Mode:          params.Mode,
			Rules:         params.Rules,
			Global:        params.Global,
			Response:      params.Response,
			FailurePolicy: params.FailurePolicy,
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
