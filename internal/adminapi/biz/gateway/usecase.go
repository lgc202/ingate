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
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

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
func (u *Usecase) List(ctx context.Context) ([]resource.Gateway, error) {
	return u.repository.List(ctx)
}

// Get 查询单个 Gateway
func (u *Usecase) Get(ctx context.Context, gatewayID string) (*resource.Gateway, error) {
	return u.repository.Get(ctx, gatewayID)
}

// Create 创建 Gateway
func (u *Usecase) Create(ctx context.Context, spec resource.GatewaySpec) (string, error) {
	u.writeMu.Lock()
	defer u.writeMu.Unlock()

	spec.Enabled = true
	if err := u.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	if err := u.validateGateway(ctx, spec, ""); err != nil {
		return "", err
	}

	id := uuid.NewString()
	if err := u.repository.Create(ctx, id, spec); err != nil {
		return "", err
	}
	return id, nil
}

// Update 更新控制台可表达的 Gateway 配置，启停状态只由 SetEnabled 修改
func (u *Usecase) Update(ctx context.Context, gatewayID, version string, submitted resource.GatewaySpec) error {
	u.writeMu.Lock()
	defer u.writeMu.Unlock()

	current, err := u.repository.Get(ctx, gatewayID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return biz.NewUserError(fmt.Sprintf("网关 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if !usesSharedHostBindings(current.Spec) {
		return biz.NewUserError("该网关包含控制台暂不支持的入口域名配置，请通过声明式 API 修改")
	}
	if err := u.validateNameUnique(ctx, submitted.DisplayName, gatewayID); err != nil {
		return err
	}

	submitted.Enabled = current.Spec.Enabled
	if err := u.validateGateway(ctx, submitted, gatewayID); err != nil {
		return err
	}
	if err := u.repository.Update(ctx, gatewayID, current.Generation, submitted); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("网关 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// SetEnabled 更新 Gateway 启停状态
func (u *Usecase) SetEnabled(ctx context.Context, gatewayID string, enabled bool) error {
	u.writeMu.Lock()
	defer u.writeMu.Unlock()

	current, err := u.repository.Get(ctx, gatewayID)
	if err != nil {
		return err
	}
	next := current.Spec
	next.Enabled = enabled
	if err := u.validateGateway(ctx, next, gatewayID); err != nil {
		return err
	}
	if err := u.repository.Update(ctx, gatewayID, current.Generation, next); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("网关 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// Delete 删除 Gateway，仍有关联路由或策略时拒绝删除
func (u *Usecase) Delete(ctx context.Context, gatewayID string) error {
	current, err := u.repository.Get(ctx, gatewayID)
	if err != nil {
		return err
	}
	routes, err := u.routes.List(ctx)
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
	usage, err := u.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindGateway, Name: gatewayID})
	if err != nil {
		return err
	}
	if usage != nil {
		return biz.NewUserError(fmt.Sprintf("网关 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	return u.repository.Delete(ctx, gatewayID)
}
