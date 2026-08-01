// Package tokenquotapolicy 实现 TokenQuotaPolicy 管理用例
package tokenquotapolicy

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	"github.com/lgc202/ingate/internal/admin/service/policytarget"
	gatewaystore "github.com/lgc202/ingate/internal/admin/store/gateway"
	routestore "github.com/lgc202/ingate/internal/admin/store/route"
	tokenquotapolicystore "github.com/lgc202/ingate/internal/admin/store/tokenquotapolicy"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const policyNotFoundMessage = "Token 配额策略不存在"

// Service 承载 TokenQuotaPolicy 管理用例
type Service struct {
	store   *tokenquotapolicystore.Store
	targets *policytarget.Resolver
	writeMu sync.Mutex
}

// New 创建 TokenQuotaPolicy service
func New(store *tokenquotapolicystore.Store, gateways *gatewaystore.Store, routes *routestore.Store) *Service {
	return &Service{store: store, targets: policytarget.New(gateways, routes)}
}

// List 查询 TokenQuotaPolicy 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	policies, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, tokenQuotaPolicyTargetRefs(policies.Items))
	if err != nil {
		return nil, err
	}
	return &ListResult{Policies: policies.Items, TargetNames: targetNames}, nil
}

// Get 查询单个 TokenQuotaPolicy
func (s *Service) Get(ctx context.Context, policyID string) (*PolicyResult, error) {
	policy, err := s.store.Get(ctx, policyID)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, xerrors.NewUserError(policyNotFoundMessage)
		}
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	return &PolicyResult{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建 TokenQuotaPolicy
func (s *Service) Create(ctx context.Context, spec resource.TokenQuotaPolicySpec) (string, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	policy := &resource.TokenQuotaPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindTokenQuotaPolicy),
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

// Update 更新 TokenQuotaPolicy
func (s *Service) Update(ctx context.Context, policyID, version string, spec resource.TokenQuotaPolicySpec) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if version == "" {
		return xerrors.NewUserError("Token 配额策略版本不能为空")
	}

	// Generation 只在期望配置变化时递增，status 更新造成的 ResourceVersion 冲突可以安全重试
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.store.Get(ctx, policyID)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return xerrors.NewUserError(policyNotFoundMessage)
			}
			return err
		}
		if version != strconv.FormatInt(current.Generation, 10) {
			return xerrors.NewUserError(fmt.Sprintf("Token 配额策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
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
		if apierrors.IsNotFound(err) {
			return xerrors.NewUserError(policyNotFoundMessage)
		}
		return err
	})
}

// SetEnabled 设置 TokenQuotaPolicy 启用状态
func (s *Service) SetEnabled(ctx context.Context, policyID string, enabled bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.store.Get(ctx, policyID)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return xerrors.NewUserError(policyNotFoundMessage)
			}
			return err
		}
		next := current.DeepCopy()
		next.Spec.Enabled = enabled
		_, err = s.store.Update(ctx, next)
		if apierrors.IsNotFound(err) {
			return xerrors.NewUserError(policyNotFoundMessage)
		}
		return err
	})
}

// Delete 删除 TokenQuotaPolicy
func (s *Service) Delete(ctx context.Context, policyID string) error {
	err := s.store.Delete(ctx, policyID)
	if apierrors.IsNotFound(err) {
		return xerrors.NewUserError(policyNotFoundMessage)
	}
	return err
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
			return xerrors.NewUserError(fmt.Sprintf("Token 配额策略名称 %q 已存在", name))
		}
	}
	return nil
}

func tokenQuotaPolicyTargetRefs(policies []resource.TokenQuotaPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}
