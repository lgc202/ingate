package gateway

import (
	"context"
	"fmt"
	"slices"

	"github.com/lgc202/ingate/internal/adminapi/pkg/apperror"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

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
	return &ListResult{
		Gateways:      gateways.Items,
		RuntimeGroups: runtimeGroupOptions(),
	}, nil
}

// Get 查询单个 Gateway
func (s *Service) Get(ctx context.Context, name string) (*GatewayResult, error) {
	gateway, err := s.store.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return &GatewayResult{
		Gateway:       gateway,
		RuntimeGroups: runtimeGroupOptions(),
	}, nil
}

// FormOptions 查询 Gateway 表单选项
func (s *Service) FormOptions(ctx context.Context) (*FormOptionsResult, error) {
	return &FormOptionsResult{
		RuntimeGroups: runtimeGroupOptions(),
		Certificates:  []CertificateOption{},
	}, nil
}

// Create 创建 Gateway
func (s *Service) Create(ctx context.Context, params CreateGatewayParams) error {
	if _, err := s.store.Get(ctx, params.Name); err == nil {
		return apierrors.NewAlreadyExists(resource.Resource(resource.ResourceGateways), params.Name)
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	gateway := gatewayResource(params.Name, "", params.Description, params.RuntimeGroup, true, params.Listeners, params.HostBindings)
	if err := s.validateDefaultGateway(ctx, gateway, ""); err != nil {
		return err
	}
	_, err := s.store.Create(ctx, gateway)
	return err
}

// Update 更新 Gateway
func (s *Service) Update(ctx context.Context, name string, params UpdateGatewayParams) error {
	current, err := s.store.Get(ctx, name)
	if err != nil {
		return err
	}
	if err := validateVersion(resource.ResourceGateways, name, params.Version, current.ResourceVersion); err != nil {
		return err
	}

	next := current.DeepCopy()
	next.Spec = gatewaySpec(params.Description, params.RuntimeGroup, current.Spec.Enabled, params.Listeners, params.HostBindings)
	if err := s.validateDefaultGateway(ctx, next, name); err != nil {
		return err
	}
	_, err = s.store.Update(ctx, next)
	return err
}

// SetEnabled 更新 Gateway 启停状态
func (s *Service) SetEnabled(ctx context.Context, name string, enabled bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.store.Get(ctx, name)
		if err != nil {
			return err
		}

		next := current.DeepCopy()
		next.Spec.Enabled = enabled
		if err := s.validateDefaultGateway(ctx, next, name); err != nil {
			return err
		}

		_, err = s.store.Update(ctx, next)
		return err
	})
}

// Delete 删除 Gateway，仍有关联路由时拒绝删除
func (s *Service) Delete(ctx context.Context, name string) error {
	routes, err := s.routes.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes.Items {
		if slices.Contains(route.Spec.ParentRefs, name) {
			return apperror.NewBadRequest(fmt.Sprintf("gateway %q still has attached routes", name))
		}
	}
	return s.store.Delete(ctx, name)
}

func gatewayResource(name, version, description, runtimeGroup string, enabled bool, listeners []ListenerParams, bindings []HostBindingParams) *resource.Gateway {
	return &resource.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindGateway),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: version,
		},
		Spec: gatewaySpec(description, runtimeGroup, enabled, listeners, bindings),
	}
}

func gatewaySpec(description, runtimeGroup string, enabled bool, listeners []ListenerParams, bindings []HostBindingParams) resource.GatewaySpec {
	return resource.GatewaySpec{
		Description:     description,
		Enabled:         enabled,
		RuntimeGroupRef: resource.RuntimeGroupRef{Name: runtimeGroupName(runtimeGroup)},
		Listeners:       resourceListeners(listeners),
		HostBindings:    resourceHostBindings(bindings),
	}
}

func resourceListeners(items []ListenerParams) []resource.Listener {
	listeners := make([]resource.Listener, 0, len(items))
	for _, item := range items {
		listeners = append(listeners, resource.Listener{
			Name:     item.Name,
			Protocol: item.Protocol,
			Port:     item.Port,
		})
	}
	return listeners
}

func resourceHostBindings(items []HostBindingParams) []resource.HostBinding {
	bindings := make([]resource.HostBinding, 0, len(items))
	for _, item := range items {
		binding := resource.HostBinding{
			Hostname:     item.Hostname,
			ListenerRefs: append([]string(nil), item.ListenerRefs...),
		}
		if item.CertificateRef != "" {
			binding.TLS = &resource.GatewayTLS{CertificateRef: item.CertificateRef}
		}
		bindings = append(bindings, binding)
	}
	return bindings
}

func (s *Service) validateDefaultGateway(ctx context.Context, gateway *resource.Gateway, excludeName string) error {
	if !gateway.Spec.Enabled || !gatewayHasCatchAllHost(gateway) {
		return nil
	}

	gateways, err := s.store.List(ctx)
	if err != nil {
		return err
	}

	for _, current := range gateways.Items {
		if current.Name == excludeName || !current.Spec.Enabled || !gatewayHasCatchAllHost(&current) {
			continue
		}
		if protocol, port, ok := sharedListener(gateway.Spec.Listeners, current.Spec.Listeners); ok {
			return apperror.NewBadRequest(fmt.Sprintf("运行入口 %s:%d 已有不限制 Host 的网关 %q；请指定 Host，或先停用/删除该网关", protocol, port, current.Name))
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
		protocol resource.ListenerProtocol
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

func validateVersion(resourceName resource.ResourceName, name, submittedVersion, currentVersion string) error {
	if submittedVersion == "" || submittedVersion == currentVersion {
		return nil
	}
	return apierrors.NewConflict(
		resource.Resource(resourceName),
		name,
		fmt.Errorf("resource version changed, current version is %s", currentVersion),
	)
}

func runtimeGroupOptions() []RuntimeGroupOption {
	return []RuntimeGroupOption{
		{
			ID:   DefaultRuntimeGroupID,
			Name: defaultRuntimeGroupName,
		},
	}
}

func runtimeGroupName(runtimeGroup string) string {
	if runtimeGroup == "" {
		return DefaultRuntimeGroupID
	}
	return runtimeGroup
}
