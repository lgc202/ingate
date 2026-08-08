package biz

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// UpstreamUsecase 承载 Upstream 查询用例
type UpstreamUsecase struct {
	repository UpstreamRepository
	routes     RouteRepository
}

// NewUpstreamUsecase 创建服务管理用例
func NewUpstreamUsecase(repository UpstreamRepository, routes RouteRepository) *UpstreamUsecase {
	return &UpstreamUsecase{repository: repository, routes: routes}
}

// List 查询 Upstream 列表
func (s *UpstreamUsecase) List(ctx context.Context) ([]resource.Upstream, error) {
	upstreams, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	return upstreams.Items, nil
}

// Get 查询单个 Upstream
func (s *UpstreamUsecase) Get(ctx context.Context, upstreamID string) (*resource.Upstream, error) {
	upstream, err := s.repository.Get(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	return upstream, nil
}

// Create 创建 Upstream
func (s *UpstreamUsecase) Create(ctx context.Context, spec resource.UpstreamSpec) (string, error) {
	if err := s.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	upstream := &resource.Upstream{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindUpstream),
		},
		ObjectMeta: metav1.ObjectMeta{Name: uuid.NewString()},
		Spec:       spec,
	}
	created, err := s.repository.Create(ctx, upstream)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 Upstream
func (s *UpstreamUsecase) Update(
	ctx context.Context,
	upstreamID string,
	version string,
	spec resource.UpstreamSpec,
	removeAPIKey bool,
) error {
	if version == "" {
		return NewUserError("服务版本不能为空")
	}

	// Generation 只随配置变化，重试时重新读取对象以避开 Controller 更新 status 引起的写冲突
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.repository.Get(ctx, upstreamID)
		if err != nil {
			return err
		}
		if version != strconv.FormatInt(current.Generation, 10) {
			return NewUserError(fmt.Sprintf("服务 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		if err := s.validateNameUnique(ctx, spec.DisplayName, upstreamID); err != nil {
			return err
		}

		next := current.DeepCopy()
		currentAuthentication := next.Spec.Authentication
		next.Spec = spec
		if removeAPIKey {
			next.Spec.Authentication = nil
		} else if next.Spec.Authentication == nil {
			next.Spec.Authentication = currentAuthentication
		}
		if err := validateAuthentication(next); err != nil {
			return err
		}
		if err := s.validateRouteCompatibility(ctx, upstreamID, next); err != nil {
			return err
		}
		_, err = s.repository.Update(ctx, next)
		return err
	})
}

func (s *UpstreamUsecase) validateRouteCompatibility(
	ctx context.Context,
	upstreamID string,
	next *resource.Upstream,
) error {
	routes, err := s.routes.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes.Items {
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.UpstreamRefs {
				if ref.Name == upstreamID && (next.Spec.Type == resource.UpstreamTypeModel || next.Spec.Protocol != resource.UpstreamProtocolHTTP) {
					return NewUserError(fmt.Sprintf("服务仍被普通路由 %q 引用，不能改为模型服务", routeDisplayName(route)))
				}
			}
			if rule.ModelRouting == nil {
				continue
			}
			for _, model := range rule.ModelRouting.Models {
				if model.UpstreamRef != upstreamID {
					continue
				}
				if !validModelUpstream(next) {
					return NewUserError(fmt.Sprintf("服务仍被模型路由 %q 引用，必须保持为有效的大模型服务", routeDisplayName(route)))
				}
				upstreamModel := model.UpstreamModel
				if upstreamModel == "" {
					upstreamModel = model.Model
				}
				if !enabledModel(next.Spec.Model, upstreamModel) {
					return NewUserError(fmt.Sprintf("服务仍被模型路由 %q 的公开模型 %q 引用，不能删除或禁用厂商模型 %q", routeDisplayName(route), model.Model, upstreamModel))
				}
			}
		}
	}
	return nil
}

// Delete 删除 Upstream，仍有关联路由时拒绝删除
func (s *UpstreamUsecase) Delete(ctx context.Context, upstreamID string) error {
	routes, err := s.routes.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes.Items {
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.UpstreamRefs {
				if ref.Name == upstreamID {
					return NewUserError(fmt.Sprintf("服务 %q 仍被路由 %q 引用", upstreamID, routeDisplayName(route)))
				}
			}
			if rule.ModelRouting != nil {
				for _, model := range rule.ModelRouting.Models {
					if model.UpstreamRef == upstreamID {
						return NewUserError(fmt.Sprintf("服务 %q 仍被路由 %q 的公开模型 %q 引用", upstreamID, routeDisplayName(route), model.Model))
					}
				}
			}
		}
	}
	return s.repository.Delete(ctx, upstreamID)
}

func routeDisplayName(route resource.Route) string {
	if route.Spec.DisplayName != "" {
		return route.Spec.DisplayName
	}
	return route.Name
}

func (s *UpstreamUsecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	upstreams, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range upstreams.Items {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return NewUserError(fmt.Sprintf("服务名称 %q 已存在", name))
		}
	}
	return nil
}

func validateAuthentication(upstream *resource.Upstream) error {
	if upstream.Spec.Authentication == nil {
		return nil
	}
	if upstream.Spec.Type != resource.UpstreamTypeModel {
		return NewUserError("服务已配置 API Key，必须保持为大模型服务")
	}
	if upstream.Spec.TLS == nil {
		return NewUserError("服务已配置 API Key，关闭 HTTPS 前请先移除 API Key")
	}
	return nil
}
