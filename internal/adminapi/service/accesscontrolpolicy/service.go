package accesscontrolpolicy

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	accesscontrolpolicystore "github.com/lgc202/ingate/internal/adminapi/store/accesscontrolpolicy"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 AccessControlPolicy 管理用例
type Service struct {
	store   *accesscontrolpolicystore.Store
	targets *policytarget.Resolver
	writeMu sync.Mutex
}

// New 创建 AccessControlPolicy service
func New(store *accesscontrolpolicystore.Store, gateways *gatewaystore.Store, routes *routestore.Store) *Service {
	return &Service{store: store, targets: policytarget.New(gateways, routes)}
}

// List 查询 AccessControlPolicy 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	policies, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, accessControlPolicyTargetRefs(policies.Items))
	if err != nil {
		return nil, err
	}
	return &ListResult{Policies: policies.Items, TargetNames: targetNames}, nil
}

// Get 查询单个 AccessControlPolicy
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

// Create 创建 AccessControlPolicy
func (s *Service) Create(ctx context.Context, params CreatePolicyParams) (string, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.validateNameUnique(ctx, params.Name, ""); err != nil {
		return "", err
	}
	policy := policyResource(uuid.NewString(), "", params.PolicyParams)
	if err := s.targets.Validate(ctx, policy.Spec.TargetRefs); err != nil {
		return "", err
	}
	created, err := s.store.Create(ctx, policy)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 AccessControlPolicy
func (s *Service) Update(ctx context.Context, policyID string, params UpdatePolicyParams) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	current, err := s.store.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if err := validateVersion(current.Spec.DisplayName, params.Version, current.ResourceVersion); err != nil {
		return err
	}
	if err := s.validateNameUnique(ctx, params.Name, policyID); err != nil {
		return err
	}

	next := current.DeepCopy()
	next.Spec = policyResource(next.Name, next.ResourceVersion, params.PolicyParams).Spec
	if err := s.targets.Validate(ctx, next.Spec.TargetRefs); err != nil {
		return err
	}
	_, err = s.store.Update(ctx, next)
	if apierrors.IsConflict(err) {
		return versionConflict(current.Spec.DisplayName)
	}
	return err
}

// SetEnabled 设置 AccessControlPolicy 启用状态
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

// Delete 删除 AccessControlPolicy
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
			return xerrors.NewUserError(fmt.Sprintf("访问控制策略名称 %q 已存在", name))
		}
	}
	return nil
}

func policyResource(id, version string, params PolicyParams) *resource.AccessControlPolicy {
	return &resource.AccessControlPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindAccessControlPolicy),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            id,
			ResourceVersion: version,
		},
		Spec: resource.AccessControlPolicySpec{
			DisplayName:   params.Name,
			Description:   params.Description,
			Enabled:       params.Enabled,
			TargetRefs:    targetRefs(params.Targets),
			DefaultAction: params.DefaultAction,
			Rules:         params.Rules,
			Response:      params.Response,
		},
	}
}

func targetRefs(targets []TargetParams) []resource.PolicyTargetRef {
	refs := make([]resource.PolicyTargetRef, 0, len(targets))
	for _, target := range targets {
		refs = append(refs, resource.PolicyTargetRef{Kind: target.Kind, Name: target.ID})
	}
	return refs
}

func accessControlPolicyTargetRefs(policies []resource.AccessControlPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}

func validateVersion(displayName, submittedVersion, currentVersion string) error {
	if submittedVersion == "" {
		return xerrors.NewUserError("资源版本不能为空")
	}
	if submittedVersion == currentVersion {
		return nil
	}
	return versionConflict(displayName)
}

func versionConflict(displayName string) error {
	return xerrors.NewUserError(fmt.Sprintf("访问控制策略 %q 已被更新，请刷新后重试", displayName))
}
