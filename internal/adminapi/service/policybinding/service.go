package policybinding

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	policybindingstore "github.com/lgc202/ingate/internal/adminapi/store/policybinding"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/adminapi/store/ratelimitpolicy"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 PolicyBinding 管理用例
type Service struct {
	store             *policybindingstore.Store
	gateways          *gatewaystore.Store
	routes            *routestore.Store
	rateLimitPolicies *ratelimitpolicystore.Store
}

// New 创建 PolicyBinding service
func New(store *policybindingstore.Store, gateways *gatewaystore.Store, routes *routestore.Store, rateLimitPolicies *ratelimitpolicystore.Store) *Service {
	return &Service{
		store:             store,
		gateways:          gateways,
		routes:            routes,
		rateLimitPolicies: rateLimitPolicies,
	}
}

// List 查询 PolicyBinding 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	bindings, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListResult{Bindings: bindings.Items}, nil
}

// Get 查询单个 PolicyBinding
func (s *Service) Get(ctx context.Context, bindingID string) (*BindingResult, error) {
	binding, err := s.store.Get(ctx, bindingID)
	if err != nil {
		return nil, err
	}
	return &BindingResult{Binding: binding}, nil
}

// Create 创建 PolicyBinding
func (s *Service) Create(ctx context.Context, params CreateBindingParams) (string, error) {
	if err := s.validateNameUnique(ctx, params.Name, ""); err != nil {
		return "", err
	}
	if err := s.validateRefs(ctx, params.BindingParams); err != nil {
		return "", err
	}

	created, err := s.store.Create(ctx, bindingResource(uuid.NewString(), "", params.BindingParams))
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 PolicyBinding
func (s *Service) Update(ctx context.Context, bindingID string, params UpdateBindingParams) error {
	current, err := s.store.Get(ctx, bindingID)
	if err != nil {
		return err
	}
	if err := validateVersion(resource.ResourcePolicyBindings, bindingID, params.Version, current.ResourceVersion); err != nil {
		return err
	}
	if err := s.validateNameUnique(ctx, params.Name, bindingID); err != nil {
		return err
	}
	if err := s.validateRefs(ctx, params.BindingParams); err != nil {
		return err
	}

	next := current.DeepCopy()
	next.Spec = bindingResource(next.Name, next.ResourceVersion, params.BindingParams).Spec
	_, err = s.store.Update(ctx, next)
	return err
}

// SetEnabled 设置 PolicyBinding 启用状态
func (s *Service) SetEnabled(ctx context.Context, bindingID string, enabled bool) error {
	current, err := s.store.Get(ctx, bindingID)
	if err != nil {
		return err
	}
	next := current.DeepCopy()
	next.Spec.Enabled = enabled
	_, err = s.store.Update(ctx, next)
	return err
}

// Delete 删除 PolicyBinding
func (s *Service) Delete(ctx context.Context, bindingID string) error {
	return s.store.Delete(ctx, bindingID)
}

func (s *Service) validateNameUnique(ctx context.Context, name, excludeID string) error {
	bindings, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range bindings.Items {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return xerrors.NewUserError(fmt.Sprintf("策略绑定名称 %q 已存在", name))
		}
	}
	return nil
}

func (s *Service) validateRefs(ctx context.Context, params BindingParams) error {
	if err := s.validateTargetRef(ctx, params.TargetRef); err != nil {
		return err
	}
	for _, policy := range params.Policies {
		switch policy.Kind {
		case resource.KindRateLimitPolicy:
			if _, err := s.rateLimitPolicies.Get(ctx, policy.Name); err != nil {
				if apierrors.IsNotFound(err) {
					return xerrors.NewUserError(fmt.Sprintf("限流策略 %q 不存在", policy.Name))
				}
				return err
			}
		default:
			return xerrors.NewUserError(fmt.Sprintf("不支持绑定策略类型 %q", policy.Kind))
		}
	}
	return nil
}

func (s *Service) validateTargetRef(ctx context.Context, target resource.PolicyTargetRef) error {
	switch target.Kind {
	case resource.KindGateway:
		if _, err := s.gateways.Get(ctx, target.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return xerrors.NewUserError(fmt.Sprintf("网关 %q 不存在", target.Name))
			}
			return err
		}
	case resource.KindRoute:
		route, err := s.routes.Get(ctx, target.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return xerrors.NewUserError(fmt.Sprintf("路由 %q 不存在", target.Name))
			}
			return err
		}
		if target.RuleName != "" {
			for _, rule := range route.Spec.Rules {
				if rule.Name == target.RuleName {
					return nil
				}
			}
			return xerrors.NewUserError(fmt.Sprintf("路由规则 %q 不存在", target.RuleName))
		}
	default:
		return xerrors.NewUserError("策略绑定目标只支持 Gateway 或 Route")
	}
	return nil
}

func bindingResource(id, version string, params BindingParams) *resource.PolicyBinding {
	return &resource.PolicyBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindPolicyBinding),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            id,
			ResourceVersion: version,
		},
		Spec: resource.PolicyBindingSpec{
			DisplayName: params.Name,
			Description: params.Description,
			Enabled:     params.Enabled,
			TargetRef:   params.TargetRef,
			Policies:    params.Policies,
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
