package route

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// Service 承载 Route 查询用例
type Service struct {
	store       *routestore.Store
	gateways    *gatewaystore.Store
	upstream    *upstreamstore.Store
	policyUsage *policytarget.UsageFinder
}

// New 创建 Route service
func New(
	store *routestore.Store,
	gateways *gatewaystore.Store,
	upstream *upstreamstore.Store,
	policyUsage *policytarget.UsageFinder,
) *Service {
	return &Service{
		store:       store,
		gateways:    gateways,
		upstream:    upstream,
		policyUsage: policyUsage,
	}
}

// List 查询 Route 列表
func (s *Service) List(ctx context.Context) ([]resource.Route, error) {
	routes, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return routes.Items, nil
}

// Get 查询单个 Route
func (s *Service) Get(ctx context.Context, routeID string) (*resource.Route, error) {
	route, err := s.store.Get(ctx, routeID)
	if err != nil {
		return nil, err
	}
	return route, nil
}

// Create 创建 Route
func (s *Service) Create(ctx context.Context, spec resource.RouteSpec) (string, error) {
	if err := s.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	if err := s.validateReferences(ctx, spec); err != nil {
		return "", err
	}
	route := &resource.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindRoute),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: uuid.NewString(),
		},
		Spec: spec,
	}
	_, err := s.store.Create(ctx, route)
	if apierrors.IsAlreadyExists(err) {
		return "", xerrors.NewUserError(fmt.Sprintf("路由 %q 已存在", route.Name))
	}
	if err != nil {
		return "", err
	}
	return route.Name, nil
}

// Update 更新 Route
func (s *Service) Update(ctx context.Context, routeID, version string, spec resource.RouteSpec) error {
	if version == "" {
		return xerrors.NewUserError("路由版本不能为空")
	}
	current, err := s.store.Get(ctx, routeID)
	if err != nil {
		return err
	}
	if err := validateVersion(resource.ResourceRoutes, routeID, version, current.ResourceVersion); err != nil {
		return err
	}
	if err := s.validateNameUnique(ctx, spec.DisplayName, routeID); err != nil {
		return err
	}
	if err := s.validateReferences(ctx, spec); err != nil {
		return err
	}

	next := current.DeepCopy()
	next.Spec = spec
	_, err = s.store.Update(ctx, next)
	return err
}

// SetEnabled 更新 Route 启停状态
func (s *Service) SetEnabled(ctx context.Context, routeID string, enabled bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.store.Get(ctx, routeID)
		if err != nil {
			return err
		}

		next := current.DeepCopy()
		next.Spec.Enabled = enabled

		_, err = s.store.Update(ctx, next)
		return err
	})
}

// Delete 删除 Route，仍被策略应用时拒绝删除
func (s *Service) Delete(ctx context.Context, routeID string) error {
	current, err := s.store.Get(ctx, routeID)
	if err != nil {
		return err
	}
	usage, err := s.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindRoute, Name: routeID})
	if err != nil {
		return err
	}
	if usage != nil {
		return xerrors.NewUserError(fmt.Sprintf("路由 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	return s.store.Delete(ctx, routeID)
}

func (s *Service) validateNameUnique(ctx context.Context, name, excludeID string) error {
	routes, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes.Items {
		if route.Name == excludeID {
			continue
		}
		if route.Spec.DisplayName == name {
			return xerrors.NewUserError(fmt.Sprintf("路由名称 %q 已存在", name))
		}
	}
	return nil
}

func (s *Service) validateReferences(ctx context.Context, spec resource.RouteSpec) error {
	for _, parentRef := range spec.ParentRefs {
		if _, err := s.gateways.Get(ctx, parentRef.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return xerrors.NewUserError(fmt.Sprintf("关联网关 %q 不存在", parentRef.Name))
			}
			return err
		}
	}
	upstreams := make(map[string]*resource.Upstream)
	for _, rule := range spec.Rules {
		for _, ref := range rule.UpstreamRefs {
			upstream, err := s.getUpstream(ctx, upstreams, ref.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return xerrors.NewUserError(fmt.Sprintf("关联服务 %q 不存在", ref.Name))
				}
				return err
			}
			if upstream.Spec.Type == resource.UpstreamTypeModel || upstream.Spec.Protocol != resource.UpstreamProtocolHTTP {
				return xerrors.NewUserError(fmt.Sprintf("模型服务 %q 只能用于模型路由", upstreamDisplayName(upstream)))
			}
		}
		if rule.ModelRouting == nil {
			continue
		}
		for _, model := range rule.ModelRouting.Models {
			upstream, err := s.getUpstream(ctx, upstreams, model.UpstreamRef)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return xerrors.NewUserError(fmt.Sprintf("关联模型服务 %q 不存在", model.UpstreamRef))
				}
				return err
			}
			if !validModelUpstream(upstream) {
				return xerrors.NewUserError(fmt.Sprintf("关联服务 %q 不是有效的大模型服务", upstreamDisplayName(upstream)))
			}
			upstreamModel := model.UpstreamModel
			if upstreamModel == "" {
				upstreamModel = model.Model
			}
			if !enabledModel(upstream.Spec.Model, upstreamModel) {
				return xerrors.NewUserError(fmt.Sprintf("模型服务 %q 未启用厂商模型 %q", upstreamDisplayName(upstream), upstreamModel))
			}
		}
	}
	return nil
}

func (s *Service) getUpstream(
	ctx context.Context,
	upstreams map[string]*resource.Upstream,
	upstreamID string,
) (*resource.Upstream, error) {
	if upstream, ok := upstreams[upstreamID]; ok {
		return upstream, nil
	}
	upstream, err := s.upstream.Get(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	upstreams[upstreamID] = upstream
	return upstream, nil
}

func validModelUpstream(upstream *resource.Upstream) bool {
	if upstream.Spec.Type != resource.UpstreamTypeModel || upstream.Spec.Model == nil {
		return false
	}
	providerProtocol, ok := upstream.Spec.Model.Provider.Protocol()
	if !ok ||
		upstream.Spec.Protocol != providerProtocol ||
		!validAPIBasePath(upstream.Spec.Model.APIBasePath) ||
		len(upstream.Spec.Model.Models) == 0 {
		return false
	}

	enabledModels := 0
	modelNames := make(map[string]struct{}, len(upstream.Spec.Model.Models))
	for _, model := range upstream.Spec.Model.Models {
		if model.Name == "" || strings.TrimSpace(model.Name) != model.Name ||
			model.DisplayName == "" || strings.TrimSpace(model.DisplayName) != model.DisplayName {
			return false
		}
		if _, exists := modelNames[model.Name]; exists {
			return false
		}
		modelNames[model.Name] = struct{}{}
		if model.Enabled {
			enabledModels++
		}
	}
	return enabledModels > 0
}

func enabledModel(modelSpec *resource.ModelSpec, name string) bool {
	for _, model := range modelSpec.Models {
		if model.Name == name {
			return model.Enabled
		}
	}
	return false
}

func validAPIBasePath(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || !strings.HasPrefix(value, "/") {
		return false
	}
	if value != "/" && strings.HasSuffix(value, "/") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != value {
		return false
	}
	return path.Clean(value) == value
}

func upstreamDisplayName(upstream *resource.Upstream) string {
	if upstream.Spec.DisplayName != "" {
		return upstream.Spec.DisplayName
	}
	return upstream.Name
}

func validateVersion(resourceName resource.ResourceName, name, submittedVersion, currentVersion string) error {
	if submittedVersion == currentVersion {
		return nil
	}
	return xerrors.NewUserError(fmt.Sprintf("%s %q 已被更新，请刷新后重试", resourceName, name))
}
