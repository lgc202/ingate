package gateway

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const noExcludedGatewayID = ""

// Service 承载 Gateway 管理用例
type Service struct {
	store  *gatewaystore.Store
	routes *routestore.Store
}

// New 创建 Gateway service
func New(store *gatewaystore.Store, routes *routestore.Store) *Service {
	return &Service{store: store, routes: routes}
}

// List 查询 Gateway 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	gateways, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListResult{Gateways: gateways.Items}, nil
}

// Get 查询单个 Gateway
func (s *Service) Get(ctx context.Context, gatewayID string) (*GatewayResult, error) {
	gateway, err := s.store.Get(ctx, gatewayID)
	if err != nil {
		return nil, err
	}
	return &GatewayResult{
		Gateway: gateway,
	}, nil
}

// Create 创建 Gateway
func (s *Service) Create(ctx context.Context, params CreateGatewayParams) (string, error) {
	if err := s.validateNameUnique(ctx, params.Name, noExcludedGatewayID); err != nil {
		return "", err
	}
	gateway := &resource.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindGateway),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: uuid.NewString(),
		},
		Spec: gatewaySpec(params.GatewayParams, true),
	}
	if err := s.validateGateway(ctx, gateway, noExcludedGatewayID); err != nil {
		return "", err
	}

	created, err := s.store.Create(ctx, gateway)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 Gateway
func (s *Service) Update(ctx context.Context, gatewayID string, params UpdateGatewayParams) error {
	current, err := s.store.Get(ctx, gatewayID)
	if err != nil {
		return err
	}
	if params.Version == "" {
		return xerrors.NewUserError("网关版本不能为空")
	}
	if params.Version != current.ResourceVersion {
		return xerrors.NewUserError(fmt.Sprintf("%s %q 已被更新，请刷新后重试", resource.ResourceGateways, gatewayID))
	}
	if err := s.validateNameUnique(ctx, params.Name, gatewayID); err != nil {
		return err
	}
	next := current.DeepCopy()
	next.Spec = gatewaySpec(params.GatewayParams, current.Spec.Enabled)
	if err := s.validateGateway(ctx, next, gatewayID); err != nil {
		return err
	}
	_, err = s.store.Update(ctx, next)
	return err
}

// SetEnabled 更新 Gateway 启停状态
func (s *Service) SetEnabled(ctx context.Context, gatewayID string, enabled bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.store.Get(ctx, gatewayID)
		if err != nil {
			return err
		}

		next := current.DeepCopy()
		next.Spec.Enabled = enabled
		if err := s.validateGateway(ctx, next, gatewayID); err != nil {
			return err
		}

		_, err = s.store.Update(ctx, next)
		return err
	})
}

// Delete 删除 Gateway，仍有关联路由时拒绝删除
func (s *Service) Delete(ctx context.Context, gatewayID string) error {
	current, err := s.store.Get(ctx, gatewayID)
	if err != nil {
		return err
	}
	routes, err := s.routes.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes.Items {
		if slices.ContainsFunc(route.Spec.ParentRefs, func(parentRef resource.ParentRef) bool {
			return parentRef.Name == gatewayID
		}) {
			return xerrors.NewUserError(fmt.Sprintf("网关 %q 仍有关联路由", current.Spec.DisplayName))
		}
	}
	return s.store.Delete(ctx, gatewayID)
}

func gatewaySpec(params GatewayParams, enabled bool) resource.GatewaySpec {
	resourceListeners := make([]resource.Listener, 0, len(params.Listeners))
	for _, listener := range params.Listeners {
		resourceListeners = append(resourceListeners, resource.Listener{
			Name:     listener.Name,
			Protocol: listener.Protocol,
			Port:     listener.Port,
		})
	}

	resourceHostBindings := make([]resource.HostBinding, 0, len(params.HostBindings))
	for _, item := range params.HostBindings {
		binding := resource.HostBinding{
			Hostname:     item.Hostname,
			ListenerRefs: append([]string(nil), item.ListenerRefs...),
		}
		if item.CertificateRef != "" {
			binding.TLS = &resource.GatewayTLS{CertificateRef: item.CertificateRef}
		}
		resourceHostBindings = append(resourceHostBindings, binding)
	}

	return resource.GatewaySpec{
		DisplayName:  params.Name,
		Description:  params.Description,
		Enabled:      enabled,
		Listeners:    resourceListeners,
		HostBindings: resourceHostBindings,
	}
}

func (s *Service) validateNameUnique(ctx context.Context, name, excludeID string) error {
	gateways, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, gateway := range gateways.Items {
		if gateway.Name == excludeID {
			continue
		}
		if gateway.Spec.DisplayName == name {
			return xerrors.NewUserError(fmt.Sprintf("网关名称 %q 已存在", name))
		}
	}
	return nil
}

func (s *Service) validateGateway(ctx context.Context, gateway *resource.Gateway, excludeID string) error {
	if !gateway.Spec.Enabled || !gatewayHasCatchAllHost(gateway) {
		return nil
	}

	gateways, err := s.store.List(ctx)
	if err != nil {
		return err
	}

	for _, current := range gateways.Items {
		if current.Name == excludeID || !current.Spec.Enabled || !gatewayHasCatchAllHost(&current) {
			continue
		}
		if protocol, port, ok := sharedListener(gateway.Spec.Listeners, current.Spec.Listeners); ok {
			return xerrors.NewUserError(fmt.Sprintf("运行入口 %s:%d 已有不限制 Host 的网关 %q；请指定 Host，或先停用/删除该网关", protocol, port, current.Spec.DisplayName))
		}
	}
	return nil
}

func gatewayHasCatchAllHost(gateway *resource.Gateway) bool {
	if len(gateway.Spec.HostBindings) == 0 {
		return true
	}
	for _, binding := range gateway.Spec.HostBindings {
		if binding.Hostname == "" {
			return true
		}
	}
	return false
}

func sharedListener(a, b []resource.Listener) (string, int, bool) {
	type listenerKey struct {
		protocol resource.Protocol
		port     int
	}

	keys := make(map[listenerKey]struct{}, len(a))
	for _, listener := range a {
		keys[listenerKey{protocol: listener.Protocol, port: listener.Port}] = struct{}{}
	}
	for _, listener := range b {
		key := listenerKey{protocol: listener.Protocol, port: listener.Port}
		if _, ok := keys[key]; ok {
			return string(key.protocol), key.port, true
		}
	}
	return "", 0, false
}
