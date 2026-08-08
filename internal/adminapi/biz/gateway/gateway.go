// Package gateway 实现 Gateway 管理用例
package gateway

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const noExcludedGatewayID = ""

// ProviderSet 提供 Gateway 管理用例
var ProviderSet = wire.NewSet(NewUsecase)

// Repository 定义 Gateway 用例需要的持久化能力
type Repository interface {
	List(context.Context) ([]resource.Gateway, error)
	Get(context.Context, string) (*resource.Gateway, error)
	Create(context.Context, string, resource.GatewaySpec) error
	Update(context.Context, string, int64, resource.GatewaySpec) error
	Delete(context.Context, string) error
}

// RouteRepository 定义删除 Gateway 时需要的 Route 查询能力
type RouteRepository interface {
	List(context.Context) ([]resource.Route, error)
}

// CertificateRepository 定义 Gateway 校验证书引用时需要的查询能力
type CertificateRepository interface {
	Get(context.Context, string) (*resource.Certificate, error)
}

// Usecase 承载 Gateway 管理用例
type Usecase struct {
	repository   Repository
	routes       RouteRepository
	certificates CertificateRepository
	policyUsage  *biz.PolicyUsageFinder
	// writeMu 保证当前 Usecase 实例内跨 Gateway 的读取校验和写入连续执行
	writeMu sync.Mutex
}

// NewUsecase 创建网关管理用例
func NewUsecase(
	repository Repository,
	routes RouteRepository,
	certificates CertificateRepository,
	policyUsage *biz.PolicyUsageFinder,
) *Usecase {
	return &Usecase{
		repository:   repository,
		routes:       routes,
		certificates: certificates,
		policyUsage:  policyUsage,
	}
}

// List 查询 Gateway 列表
func (s *Usecase) List(ctx context.Context) ([]resource.Gateway, error) {
	gateways, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	return gateways, nil
}

// Get 查询单个 Gateway
func (s *Usecase) Get(ctx context.Context, gatewayID string) (*resource.Gateway, error) {
	gateway, err := s.repository.Get(ctx, gatewayID)
	if err != nil {
		return nil, err
	}
	return gateway, nil
}

// Create 创建 Gateway
func (s *Usecase) Create(ctx context.Context, spec resource.GatewaySpec) (string, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	spec.Enabled = true
	if err := s.validateNameUnique(ctx, spec.DisplayName, noExcludedGatewayID); err != nil {
		return "", err
	}
	if err := s.validateGateway(ctx, spec, noExcludedGatewayID); err != nil {
		return "", err
	}

	id := uuid.NewString()
	if err := s.repository.Create(ctx, id, spec); err != nil {
		return "", err
	}
	return id, nil
}

// Update 更新 Gateway
func (s *Usecase) Update(ctx context.Context, gatewayID, version string, spec resource.GatewaySpec) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	current, err := s.repository.Get(ctx, gatewayID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return biz.NewUserError(fmt.Sprintf("网关 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if err := s.validateNameUnique(ctx, spec.DisplayName, gatewayID); err != nil {
		return err
	}

	spec.Enabled = current.Spec.Enabled
	if err := s.validateGateway(ctx, spec, gatewayID); err != nil {
		return err
	}
	if err := s.repository.Update(ctx, gatewayID, current.Generation, spec); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("网关 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// SetEnabled 更新 Gateway 启停状态
func (s *Usecase) SetEnabled(ctx context.Context, gatewayID string, enabled bool) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	current, err := s.repository.Get(ctx, gatewayID)
	if err != nil {
		return err
	}
	spec := current.Spec
	spec.Enabled = enabled
	if err := s.validateGateway(ctx, spec, gatewayID); err != nil {
		return err
	}
	if err := s.repository.Update(ctx, gatewayID, current.Generation, spec); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("网关 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// Delete 删除 Gateway，仍有关联路由时拒绝删除
func (s *Usecase) Delete(ctx context.Context, gatewayID string) error {
	current, err := s.repository.Get(ctx, gatewayID)
	if err != nil {
		return err
	}
	routes, err := s.routes.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes {
		if slices.ContainsFunc(route.Spec.ParentRefs, func(parentRef resource.ParentRef) bool {
			return parentRef.Name == gatewayID
		}) {
			return biz.NewUserError(fmt.Sprintf("网关 %q 仍有关联路由", current.Spec.DisplayName))
		}
	}
	usage, err := s.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindGateway, Name: gatewayID})
	if err != nil {
		return err
	}
	if usage != nil {
		return biz.NewUserError(fmt.Sprintf("网关 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	return s.repository.Delete(ctx, gatewayID)
}

func (s *Usecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	gateways, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, gateway := range gateways {
		if gateway.Name == excludeID {
			continue
		}
		if gateway.Spec.DisplayName == name {
			return biz.NewUserError(fmt.Sprintf("网关名称 %q 已存在", name))
		}
	}
	return nil
}

func (s *Usecase) validateGateway(ctx context.Context, spec resource.GatewaySpec, excludeID string) error {
	if !spec.Enabled {
		return nil
	}
	if err := s.validateCertificateRefs(ctx, spec); err != nil {
		return err
	}

	gateways, err := s.repository.List(ctx)
	if err != nil {
		return err
	}

	for _, current := range gateways {
		if current.Name == excludeID || !current.Spec.Enabled {
			continue
		}

		for _, listener := range spec.Listeners {
			for _, currentListener := range current.Spec.Listeners {
				if listener.Port != currentListener.Port {
					continue
				}
				if listener.Protocol != currentListener.Protocol {
					return biz.NewUserError(fmt.Sprintf(
						"端口 %d 已被网关 %q 的 %s 入口占用，不能同时配置为 %s 入口",
						listener.Port,
						current.Spec.DisplayName,
						currentListener.Protocol,
						listener.Protocol,
					))
				}

				for _, hostname := range listenerHostnames(spec, listener.Name) {
					for _, currentHostname := range listenerHostnames(current.Spec, currentListener.Name) {
						if !hostnameutil.Overlaps(hostname, currentHostname) {
							continue
						}
						return biz.NewUserError(fmt.Sprintf(
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

func (s *Usecase) validateCertificateRefs(ctx context.Context, spec resource.GatewaySpec) error {
	seen := make(map[string]struct{})
	for _, listener := range spec.Listeners {
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
		if errors.Is(err, biz.ErrResourceNotFound) {
			return biz.NewUserError(fmt.Sprintf("HTTPS 证书 %q 不存在", listener.CertificateRef))
		}
		return err
	}
	return nil
}

func listenerHostnames(spec resource.GatewaySpec, listenerName string) []string {
	var hostnames []string
	for _, binding := range spec.HostBindings {
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
