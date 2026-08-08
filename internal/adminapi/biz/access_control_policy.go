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

// AccessControlPolicyUsecase 承载 AccessControlPolicy 管理用例
type AccessControlPolicyUsecase struct {
	repository AccessControlPolicyRepository
	targets    *PolicyTargetResolver
	writeMu    sync.Mutex
}

// NewAccessControlPolicyUsecase 创建访问控制策略用例
func NewAccessControlPolicyUsecase(
	repository AccessControlPolicyRepository,
	gateways GatewayRepository,
	routes RouteRepository,
) *AccessControlPolicyUsecase {
	return &AccessControlPolicyUsecase{repository: repository, targets: NewPolicyTargetResolver(gateways, routes)}
}

// List 查询 AccessControlPolicy 列表
func (s *AccessControlPolicyUsecase) List(ctx context.Context) (*AccessControlPolicyList, error) {
	policies, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, accessControlPolicyTargetRefs(policies.Items))
	if err != nil {
		return nil, err
	}
	return &AccessControlPolicyList{Policies: policies.Items, TargetNames: targetNames}, nil
}

// Get 查询单个 AccessControlPolicy
func (s *AccessControlPolicyUsecase) Get(ctx context.Context, policyID string) (*AccessControlPolicyResult, error) {
	policy, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	return &AccessControlPolicyResult{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建 AccessControlPolicy
func (s *AccessControlPolicyUsecase) Create(ctx context.Context, spec resource.AccessControlPolicySpec) (string, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	policy := &resource.AccessControlPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindAccessControlPolicy),
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

// Update 更新 AccessControlPolicy
func (s *AccessControlPolicyUsecase) Update(ctx context.Context, policyID, version string, spec resource.AccessControlPolicySpec) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if version == "" {
		return NewUserError("访问控制策略版本不能为空")
	}

	// Generation 只在期望配置变化时递增，status 更新造成的 ResourceVersion 冲突可以安全重试
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.repository.Get(ctx, policyID)
		if err != nil {
			return err
		}
		if version != strconv.FormatInt(current.Generation, 10) {
			return NewUserError(fmt.Sprintf("访问控制策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
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

// SetEnabled 设置 AccessControlPolicy 启用状态
func (s *AccessControlPolicyUsecase) SetEnabled(ctx context.Context, policyID string, enabled bool) error {
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

// Delete 删除 AccessControlPolicy
func (s *AccessControlPolicyUsecase) Delete(ctx context.Context, policyID string) error {
	return s.repository.Delete(ctx, policyID)
}

func (s *AccessControlPolicyUsecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	policies, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range policies.Items {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return NewUserError(fmt.Sprintf("访问控制策略名称 %q 已存在", name))
		}
	}
	return nil
}

func accessControlPolicyTargetRefs(policies []resource.AccessControlPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}
