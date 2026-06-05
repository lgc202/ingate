package upstream

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Service 承载 Upstream 查询用例
type Service struct {
	store  *upstreamstore.Store
	routes *routestore.Store
}

// New 创建 Upstream service
func New(store *upstreamstore.Store, routes *routestore.Store) *Service {
	return &Service{store: store, routes: routes}
}

// List 查询 Upstream 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	upstreams, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	routes, err := s.routes.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListResult{
		Upstreams: upstreams.Items,
		Routes:    routes.Items,
	}, nil
}

// Get 查询单个 Upstream
func (s *Service) Get(ctx context.Context, name string) (*UpstreamResult, error) {
	upstream, err := s.store.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	routes, err := s.routes.List(ctx)
	if err != nil {
		return nil, err
	}
	return &UpstreamResult{
		Upstream: upstream,
		Routes:   routes.Items,
	}, nil
}

// Create 创建 Upstream
func (s *Service) Create(ctx context.Context, upstream *resource.Upstream) error {
	_, err := s.store.Create(ctx, upstream)
	if apierrors.IsAlreadyExists(err) {
		return xerrors.NewUserError(fmt.Sprintf("上游 %q 已存在", upstream.Name))
	}
	return err
}

// Update 更新 Upstream
func (s *Service) Update(ctx context.Context, name string, upstream *resource.Upstream) error {
	if upstream.Name != name {
		return xerrors.NewUserError("上游名称不能修改")
	}

	current, err := s.store.Get(ctx, name)
	if err != nil {
		return err
	}
	if err := validateVersion(resource.ResourceUpstreams, name, upstream.ResourceVersion, current.ResourceVersion); err != nil {
		return err
	}
	next := current.DeepCopy()
	applyUpstreamUpdate(next, upstream)
	_, err = s.store.Update(ctx, next)
	return err
}

// Delete 删除 Upstream，仍有关联路由时拒绝删除
func (s *Service) Delete(ctx context.Context, name string) error {
	routes, err := s.routes.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes.Items {
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.UpstreamRefs {
				if ref.Name == name {
					return xerrors.NewUserError(fmt.Sprintf("上游 %q 仍被路由 %q 引用", name, route.Name))
				}
			}
		}
	}
	return s.store.Delete(ctx, name)
}

func applyUpstreamUpdate(next *resource.Upstream, submitted *resource.Upstream) {
	next.Spec = submitted.Spec
	if next.Annotations == nil {
		next.Annotations = map[string]string{}
	}
	for _, key := range []string{
		resource.AnnotationUpstreamServiceType,
		resource.AnnotationUpstreamLoadBalancePolicy,
		resource.AnnotationUpstreamEndpoints,
		resource.AnnotationUpstreamHealthCheck,
	} {
		delete(next.Annotations, key)
	}
	for key, value := range submitted.Annotations {
		if key == resource.AnnotationUpstreamServiceType ||
			key == resource.AnnotationUpstreamLoadBalancePolicy ||
			key == resource.AnnotationUpstreamEndpoints ||
			key == resource.AnnotationUpstreamHealthCheck {
			next.Annotations[key] = value
		}
	}
}

func validateVersion(resourceName resource.ResourceName, name, submittedVersion, currentVersion string) error {
	if submittedVersion == "" || submittedVersion == currentVersion {
		return nil
	}
	return xerrors.NewUserError(fmt.Sprintf("%s %q 已被更新，请刷新后重试", resourceName, name))
}
