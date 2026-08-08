package biz

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// RateLimitPolicyUsecase 承载 RateLimitPolicy 管理用例
type RateLimitPolicyUsecase struct {
	repository RateLimitPolicyRepository
	targets    *PolicyTargetResolver
	writeMu    sync.Mutex
}

// NewRateLimitPolicyUsecase 创建请求限流策略用例
func NewRateLimitPolicyUsecase(
	repository RateLimitPolicyRepository,
	gateways GatewayRepository,
	routes RouteRepository,
) *RateLimitPolicyUsecase {
	return &RateLimitPolicyUsecase{repository: repository, targets: NewPolicyTargetResolver(gateways, routes)}
}

// List 查询 RateLimitPolicy 列表
func (s *RateLimitPolicyUsecase) List(ctx context.Context) (*RateLimitPolicyList, error) {
	policies, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, rateLimitPolicyTargetRefs(policies.Items))
	if err != nil {
		return nil, err
	}
	return &RateLimitPolicyList{Policies: policies.Items, TargetNames: targetNames}, nil
}

// Get 查询单个 RateLimitPolicy
func (s *RateLimitPolicyUsecase) Get(ctx context.Context, policyID string) (*RateLimitPolicyResult, error) {
	policy, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	return &RateLimitPolicyResult{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建 RateLimitPolicy
func (s *RateLimitPolicyUsecase) Create(ctx context.Context, spec resource.RateLimitPolicySpec) (string, error) {
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
	created, err := s.repository.Create(ctx, policy)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 RateLimitPolicy
func (s *RateLimitPolicyUsecase) Update(ctx context.Context, policyID, version string, spec resource.RateLimitPolicySpec) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if version == "" {
		return NewUserError("限流策略版本不能为空")
	}

	// Generation 只在期望配置变化时递增，status 更新造成的 ResourceVersion 冲突可以安全重试
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.repository.Get(ctx, policyID)
		if err != nil {
			return err
		}
		if version != strconv.FormatInt(current.Generation, 10) {
			return NewUserError(fmt.Sprintf("限流策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		if err := s.validateNameUnique(ctx, spec.DisplayName, policyID); err != nil {
			return err
		}

		next := current.DeepCopy()
		next.Spec = spec
		if err := s.targets.Validate(ctx, next.Spec.TargetRefs); err != nil {
			return err
		}
		_, err = s.repository.Update(ctx, next)
		return err
	})
}

// SetEnabled 设置 RateLimitPolicy 启用状态
func (s *RateLimitPolicyUsecase) SetEnabled(ctx context.Context, policyID string, enabled bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.repository.Get(ctx, policyID)
		if err != nil {
			return err
		}
		next := current.DeepCopy()
		next.Spec.Enabled = enabled
		_, err = s.repository.Update(ctx, next)
		return err
	})
}

// Delete 删除 RateLimitPolicy
func (s *RateLimitPolicyUsecase) Delete(ctx context.Context, policyID string) error {
	return s.repository.Delete(ctx, policyID)
}

func (s *RateLimitPolicyUsecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	policies, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range policies.Items {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return NewUserError(fmt.Sprintf("限流策略名称 %q 已存在", name))
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
