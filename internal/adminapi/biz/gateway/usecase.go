// Package gateway 处理 Gateway 的业务规则和资源协作。
package gateway

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义 Gateway 管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Gateway], error)
	Get(ctx context.Context, gatewayID string) (*resource.Gateway, error)
	Create(ctx context.Context, gatewayID string, spec resource.GatewaySpec) (*resource.Gateway, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.Gateway,
		spec resource.GatewaySpec,
	) (*resource.Gateway, error)
	Delete(ctx context.Context, observed *resource.Gateway) error
}

// RouteLister 定义 Gateway 删除检查所需的 Route 分页能力。
type RouteLister interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Route], error)
}

// CertificateReader 定义 Gateway 校验证书引用所需的批量读取能力。
type CertificateReader interface {
	ListByIDs(ctx context.Context, certificateIDs []string) (map[string]*resource.Certificate, error)
}

// Usecase 协调 Gateway 的校验、引用约束和持久化。
type Usecase struct {
	store             Store
	routes            RouteLister
	certificates      CertificateReader
	policyUsageFinder *biz.PolicyUsageFinder
}

// NewUsecase 创建 Gateway 用例。
func NewUsecase(
	store Store,
	routes RouteLister,
	certificates CertificateReader,
	policyUsageFinder *biz.PolicyUsageFinder,
) *Usecase {
	return &Usecase{
		store:             store,
		routes:            routes,
		certificates:      certificates,
		policyUsageFinder: policyUsageFinder,
	}
}

// List 返回满足筛选条件的 Gateway 列表。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (biz.PageResult[resource.Gateway], error) {
	return biz.FilterPage(ctx, page, uc.store.ListPage, func(gateway resource.Gateway) bool {
		var searchText strings.Builder
		searchText.WriteString(gateway.Spec.DisplayName)
		for _, listener := range gateway.Spec.Listeners {
			searchText.WriteByte(' ')
			searchText.WriteString(listener.Name)
			searchText.WriteByte(' ')
			searchText.WriteString(listener.Hostname)
			searchText.WriteByte(' ')
			searchText.WriteString(strconv.Itoa(listener.Port))
		}
		status := biz.EnabledResourceStatus(
			gateway.Generation,
			gateway.Spec.Enabled,
			gateway.Status.Conditions,
		)
		return filter.Match(searchText.String(), gateway.Spec.Enabled, status)
	})
}

// Get 返回指定 Gateway。
func (uc *Usecase) Get(ctx context.Context, gatewayID string) (*resource.Gateway, error) {
	return uc.store.Get(ctx, gatewayID)
}

// Create 创建 Gateway。
func (uc *Usecase) Create(ctx context.Context, spec resource.GatewaySpec) (*resource.Gateway, error) {
	if err := uc.checkCertificateReferences(ctx, spec); err != nil {
		return nil, err
	}
	if err := uc.checkListenerClaimsAvailable(ctx, "", spec); err != nil {
		return nil, err
	}

	gatewayID := uuid.NewString()
	return uc.store.Create(ctx, gatewayID, spec)
}

// Replace 使用配置版本完整替换 Gateway 配置。
func (uc *Usecase) Replace(
	ctx context.Context,
	gatewayID string,
	expectedGeneration int64,
	spec resource.GatewaySpec,
) (*resource.Gateway, error) {
	current, err := uc.store.Get(ctx, gatewayID)
	if err != nil {
		return nil, err
	}

	if current.Generation != expectedGeneration {
		return nil, biz.ErrResourceVersionConflict
	}
	if err := uc.checkCertificateReferences(ctx, spec); err != nil {
		return nil, err
	}
	if err := uc.checkListenerClaimsAvailable(ctx, gatewayID, spec); err != nil {
		return nil, err
	}

	return uc.store.ReplaceSpec(ctx, current, spec)
}

// Delete 删除 Gateway，并在写入前检查当前可见的资源引用。
func (uc *Usecase) Delete(ctx context.Context, gatewayID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, gatewayID)
	if err != nil {
		return err
	}

	if current.Generation != expectedGeneration {
		return biz.ErrResourceVersionConflict
	}
	if err := uc.checkNotReferenced(ctx, current); err != nil {
		return err
	}

	return uc.store.Delete(ctx, current)
}
