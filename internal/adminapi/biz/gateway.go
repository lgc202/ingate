package biz

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"sync"

	"github.com/google/uuid"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const noExcludedGatewayID = ""

// GatewayUsecase 承载 Gateway 管理用例
type GatewayUsecase struct {
	repository   GatewayRepository
	routes       RouteRepository
	certificates CertificateRepository
	policyUsage  *PolicyUsageFinder
	// writeMu 保证当前 GatewayUsecase 实例内跨 Gateway 的读取校验和写入连续执行
	writeMu sync.Mutex
}

// NewGatewayUsecase 创建网关管理用例
func NewGatewayUsecase(
	repository GatewayRepository,
	routes RouteRepository,
	certificates CertificateRepository,
	policyUsage *PolicyUsageFinder,
) *GatewayUsecase {
	return &GatewayUsecase{
		repository:   repository,
		routes:       routes,
		certificates: certificates,
		policyUsage:  policyUsage,
	}
}

// List 查询 Gateway 列表
func (s *GatewayUsecase) List(ctx context.Context) ([]resource.Gateway, error) {
	gateways, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	return gateways.Items, nil
}

// Get 查询单个 Gateway
func (s *GatewayUsecase) Get(ctx context.Context, gatewayID string) (*resource.Gateway, error) {
	gateway, err := s.repository.Get(ctx, gatewayID)
	if err != nil {
		return nil, err
	}
	return gateway, nil
}

// Create 创建 Gateway
func (s *GatewayUsecase) Create(ctx context.Context, spec resource.GatewaySpec) (string, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	spec.Enabled = true
	if err := s.validateNameUnique(ctx, spec.DisplayName, noExcludedGatewayID); err != nil {
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
		Spec: spec,
	}
	if err := s.validateGateway(ctx, gateway, noExcludedGatewayID); err != nil {
		return "", err
	}

	created, err := s.repository.Create(ctx, gateway)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 Gateway
func (s *GatewayUsecase) Update(ctx context.Context, gatewayID, version string, spec resource.GatewaySpec) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if version == "" {
		return NewUserError("网关版本不能为空")
	}

	// Generation 只随配置变化，重试时重新读取对象以避开 Controller 更新 status 引起的写冲突
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.repository.Get(ctx, gatewayID)
		if err != nil {
			return err
		}
		if version != strconv.FormatInt(current.Generation, 10) {
			return NewUserError(fmt.Sprintf("网关 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		if err := s.validateNameUnique(ctx, spec.DisplayName, gatewayID); err != nil {
			return err
		}

		next := current.DeepCopy()
		next.Spec = spec
		next.Spec.Enabled = current.Spec.Enabled
		if err := s.validateGateway(ctx, next, gatewayID); err != nil {
			return err
		}
		_, err = s.repository.Update(ctx, next)
		return err
	})
}

// SetEnabled 更新 Gateway 启停状态
func (s *GatewayUsecase) SetEnabled(ctx context.Context, gatewayID string, enabled bool) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.repository.Get(ctx, gatewayID)
		if err != nil {
			return err
		}

		next := current.DeepCopy()
		next.Spec.Enabled = enabled
		if err := s.validateGateway(ctx, next, gatewayID); err != nil {
			return err
		}

		_, err = s.repository.Update(ctx, next)
		return err
	})
}

// Delete 删除 Gateway，仍有关联路由时拒绝删除
func (s *GatewayUsecase) Delete(ctx context.Context, gatewayID string) error {
	current, err := s.repository.Get(ctx, gatewayID)
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
			return NewUserError(fmt.Sprintf("网关 %q 仍有关联路由", current.Spec.DisplayName))
		}
	}
	usage, err := s.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindGateway, Name: gatewayID})
	if err != nil {
		return err
	}
	if usage != nil {
		return NewUserError(fmt.Sprintf("网关 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	return s.repository.Delete(ctx, gatewayID)
}

func (s *GatewayUsecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	gateways, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, gateway := range gateways.Items {
		if gateway.Name == excludeID {
			continue
		}
		if gateway.Spec.DisplayName == name {
			return NewUserError(fmt.Sprintf("网关名称 %q 已存在", name))
		}
	}
	return nil
}

func (s *GatewayUsecase) validateGateway(ctx context.Context, gateway *resource.Gateway, excludeID string) error {
	if !gateway.Spec.Enabled {
		return nil
	}
	if err := s.validateCertificateRefs(ctx, gateway); err != nil {
		return err
	}

	gateways, err := s.repository.List(ctx)
	if err != nil {
		return err
	}

	for _, current := range gateways.Items {
		if current.Name == excludeID || !current.Spec.Enabled {
			continue
		}

		for _, listener := range gateway.Spec.Listeners {
			for _, currentListener := range current.Spec.Listeners {
				if listener.Port != currentListener.Port {
					continue
				}
				if listener.Protocol != currentListener.Protocol {
					return NewUserError(fmt.Sprintf(
						"端口 %d 已被网关 %q 的 %s 入口占用，不能同时配置为 %s 入口",
						listener.Port,
						current.Spec.DisplayName,
						currentListener.Protocol,
						listener.Protocol,
					))
				}

				for _, hostname := range listenerHostnames(gateway, listener.Name) {
					for _, currentHostname := range listenerHostnames(&current, currentListener.Name) {
						if !hostnameutil.Overlaps(hostname, currentHostname) {
							continue
						}
						return NewUserError(fmt.Sprintf(
							"访问入口 %s:%d 的域名范围 %s 与网关 %q 的域名范围 %s 重叠；请调整域名，或先停用该网关",
							listener.Protocol,
							listener.Port,
							hostClaimDescription(hostname),
							current.Spec.DisplayName,
							hostClaimDescription(currentHostname),
						))
					}
				}
			}
		}
	}
	return nil
}

func (s *GatewayUsecase) validateCertificateRefs(ctx context.Context, gateway *resource.Gateway) error {
	seen := make(map[string]struct{})
	for _, listener := range gateway.Spec.Listeners {
		if listener.Protocol != resource.ProtocolHTTPS {
			continue
		}
		if _, exists := seen[listener.CertificateRef]; exists {
			continue
		}
		seen[listener.CertificateRef] = struct{}{}

		_, err := s.certificates.Get(ctx, listener.CertificateRef)
		if err == nil {
			continue
		}
		if apierrors.IsNotFound(err) {
			return NewUserError(fmt.Sprintf("HTTPS 证书 %q 不存在", listener.CertificateRef))
		}
		return err
	}
	return nil
}

func listenerHostnames(gateway *resource.Gateway, listenerName string) []string {
	var hostnames []string
	for _, binding := range gateway.Spec.HostBindings {
		if !slices.Contains(binding.ListenerRefs, listenerName) {
			continue
		}
		hostname, ok := hostnameutil.Normalize(binding.Hostname)
		if ok {
			hostnames = append(hostnames, hostname)
		}
	}
	if len(hostnames) == 0 {
		return []string{"*"}
	}
	return hostnames
}

func hostClaimDescription(hostname string) string {
	if hostname == "*" {
		return "全部域名"
	}
	return fmt.Sprintf("%q", hostname)
}
