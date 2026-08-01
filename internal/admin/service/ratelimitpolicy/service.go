// Package ratelimitpolicy 实现 RateLimitPolicy 管理用例
package ratelimitpolicy

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	"github.com/lgc202/ingate/internal/admin/service/policytarget"
	gatewaystore "github.com/lgc202/ingate/internal/admin/store/gateway"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/admin/store/ratelimitpolicy"
	routestore "github.com/lgc202/ingate/internal/admin/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 RateLimitPolicy 管理用例
type Service struct {
	store   *ratelimitpolicystore.Store
	targets *policytarget.Resolver
	writeMu sync.Mutex
}

// New 创建 RateLimitPolicy service
func New(store *ratelimitpolicystore.Store, gateways *gatewaystore.Store, routes *routestore.Store) *Service {
	return &Service{store: store, targets: policytarget.New(gateways, routes)}
}

// List 查询 RateLimitPolicy 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	policies, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, rateLimitPolicyTargetRefs(policies.Items))
	if err != nil {
		return nil, err
	}
	return &ListResult{Policies: policies.Items, TargetNames: targetNames}, nil
}

// Get 查询单个 RateLimitPolicy
func (s *Service) Get(ctx context.Context, policyID string) (*PolicyResult, error) {
	policy, err := s.store.Get(ctx, policyID)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	return &PolicyResult{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建 RateLimitPolicy
func (s *Service) Create(ctx context.Context, spec resource.RateLimitPolicySpec) (string, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	policy := &resource.RateLimitPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindRateLimitPolicy),
		},
		ObjectMeta: metav1.ObjectMeta{Name: uuid.NewString()},
		Spec:       spec,
	}
	if err := s.targets.Validate(ctx, policy.Spec.TargetRefs); err != nil {
		return "", err
	}
	created, err := s.store.Create(ctx, policy)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 RateLimitPolicy
func (s *Service) Update(ctx context.Context, policyID, version string, spec resource.RateLimitPolicySpec) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if version == "" {
		return xerrors.NewUserError("限流策略版本不能为空")
	}

	// Generation 只在期望配置变化时递增，status 更新造成的 ResourceVersion 冲突可以安全重试
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.store.Get(ctx, policyID)
		if err != nil {
			return err
		}
		if version != strconv.FormatInt(current.Generation, 10) {
			return xerrors.NewUserError(fmt.Sprintf("限流策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		if err := s.validateNameUnique(ctx, spec.DisplayName, policyID); err != nil {
			return err
		}

		next := current.DeepCopy()
		next.Spec = spec
		if err := s.targets.Validate(ctx, next.Spec.TargetRefs); err != nil {
			return err
		}
		_, err = s.store.Update(ctx, next)
		return err
	})
}

// SetEnabled 设置 RateLimitPolicy 启用状态
func (s *Service) SetEnabled(ctx context.Context, policyID string, enabled bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.store.Get(ctx, policyID)
		if err != nil {
			return err
		}
		next := current.DeepCopy()
		next.Spec.Enabled = enabled
		_, err = s.store.Update(ctx, next)
		return err
	})
}

// Delete 删除 RateLimitPolicy
func (s *Service) Delete(ctx context.Context, policyID string) error {
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

func rateLimitPolicyTargetRefs(policies []resource.RateLimitPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}
