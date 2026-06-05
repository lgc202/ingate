package upstream

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	return &ListResult{
		Upstreams: upstreams.Items,
	}, nil
}

// Get 查询单个 Upstream
func (s *Service) Get(ctx context.Context, upstreamID string) (*UpstreamResult, error) {
	upstream, err := s.store.Get(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	return &UpstreamResult{
		Upstream: upstream,
	}, nil
}

// Create 创建 Upstream
func (s *Service) Create(ctx context.Context, params CreateUpstreamParams) (string, error) {
	if err := s.validateNameUnique(ctx, params.Name, ""); err != nil {
		return "", err
	}

	upstream := upstreamResource(uuid.NewString(), "", params.UpstreamParams)
	created, err := s.store.Create(ctx, upstream)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 Upstream
func (s *Service) Update(ctx context.Context, upstreamID string, params UpdateUpstreamParams) error {
	current, err := s.store.Get(ctx, upstreamID)
	if err != nil {
		return err
	}
	if err := validateVersion(resource.ResourceUpstreams, upstreamID, params.Version, current.ResourceVersion); err != nil {
		return err
	}
	if err := s.validateNameUnique(ctx, params.Name, upstreamID); err != nil {
		return err
	}
	next := current.DeepCopy()
	applyUpstreamParams(next, params.UpstreamParams)
	_, err = s.store.Update(ctx, next)
	return err
}

// Delete 删除 Upstream，仍有关联路由时拒绝删除
func (s *Service) Delete(ctx context.Context, upstreamID string) error {
	routes, err := s.routes.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes.Items {
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.UpstreamRefs {
				if ref.Name == upstreamID {
					return xerrors.NewUserError(fmt.Sprintf("上游 %q 仍被路由 %q 引用", upstreamID, route.Name))
				}
			}
		}
	}
	return s.store.Delete(ctx, upstreamID)
}

func (s *Service) validateNameUnique(ctx context.Context, name, excludeID string) error {
	upstreams, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range upstreams.Items {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return xerrors.NewUserError(fmt.Sprintf("上游名称 %q 已存在", name))
		}
	}
	return nil
}

func upstreamResource(id, version string, params UpstreamParams) *resource.Upstream {
	return &resource.Upstream{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindUpstream),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            id,
			ResourceVersion: version,
		},
		Spec: resource.UpstreamSpec{
			DisplayName:       params.Name,
			Type:              params.Type,
			LoadBalancePolicy: params.LoadBalancePolicy,
			HealthCheck:       params.HealthCheck,
			Endpoints:         resourceEndpoints(params.Endpoints),
		},
	}
}

func applyUpstreamParams(next *resource.Upstream, params UpstreamParams) {
	next.Spec = upstreamResource(next.Name, next.ResourceVersion, params).Spec
}

func validateVersion(resourceName resource.ResourceName, name, submittedVersion, currentVersion string) error {
	if submittedVersion == "" || submittedVersion == currentVersion {
		return nil
	}
	return xerrors.NewUserError(fmt.Sprintf("%s %q 已被更新，请刷新后重试", resourceName, name))
}

func resourceEndpoints(endpoints []EndpointParams) []resource.Endpoint {
	return lo.Map(endpoints, func(endpoint EndpointParams, _ int) resource.Endpoint {
		return resource.Endpoint{
			Name:    endpoint.ID,
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  endpoint.Weight,
			Enabled: endpoint.Enabled,
		}
	})
}
