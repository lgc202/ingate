// Package route 处理 Route 的业务规则和资源协作。
package route

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// TypeFilter 表达 Route 列表的类型筛选条件。
type TypeFilter uint8

const (
	// TypeAny 不限制 Route 类型。
	TypeAny TypeFilter = iota
	// TypeAPI 只返回普通 API Route。
	TypeAPI
	// TypeAI 只返回 AI Route。
	TypeAI
)

// Store 定义 Route 管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Route], error)
	Get(ctx context.Context, routeID string) (*resource.Route, error)
	Create(ctx context.Context, routeID string, spec resource.RouteSpec) (*resource.Route, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.Route,
		spec resource.RouteSpec,
	) (*resource.Route, error)
	Delete(ctx context.Context, observed *resource.Route) error
}

// GatewayReader 定义 Route 校验 Gateway 引用所需的批量读取能力。
type GatewayReader interface {
	ListByIDs(ctx context.Context, gatewayIDs []string) (map[string]*resource.Gateway, error)
}

// ServiceReader 定义 Route 校验目标 Service 所需的批量读取能力。
type ServiceReader interface {
	ListByIDs(ctx context.Context, serviceIDs []string) (map[string]*resource.Upstream, error)
}

// CallerLister 定义 Route 删除检查所需的 Caller 分页能力。
type CallerLister interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Caller], error)
}

// ListFilter 表达 Route 列表的筛选条件。
type ListFilter struct {
	biz.ResourceFilter
	Type TypeFilter
}

// Usecase 协调 Route 的校验、引用约束和持久化。
type Usecase struct {
	store             Store
	gateways          GatewayReader
	services          ServiceReader
	callers           CallerLister
	policyUsageFinder *biz.PolicyUsageFinder
}

// NewUsecase 创建 Route 用例。
func NewUsecase(
	store Store,
	gateways GatewayReader,
	services ServiceReader,
	callers CallerLister,
	policyUsageFinder *biz.PolicyUsageFinder,
) *Usecase {
	return &Usecase{
		store:             store,
		gateways:          gateways,
		services:          services,
		callers:           callers,
		policyUsageFinder: policyUsageFinder,
	}
}

// List 返回满足筛选条件的 Route 列表。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter ListFilter,
) (biz.PageResult[resource.Route], error) {
	return biz.FilterPage(ctx, page, uc.store.ListPage, filter.matches)
}

// Get 返回指定 Route。
func (uc *Usecase) Get(ctx context.Context, routeID string) (*resource.Route, error) {
	return uc.store.Get(ctx, routeID)
}

// Create 创建 Route。
func (uc *Usecase) Create(ctx context.Context, spec resource.RouteSpec) (*resource.Route, error) {
	if err := uc.checkReferences(ctx, spec); err != nil {
		return nil, err
	}

	routeID := uuid.NewString()
	return uc.store.Create(ctx, routeID, spec)
}

// Replace 使用配置版本完整替换 Route 配置。
func (uc *Usecase) Replace(
	ctx context.Context,
	routeID string,
	expectedGeneration int64,
	spec resource.RouteSpec,
) (*resource.Route, error) {
	current, err := uc.store.Get(ctx, routeID)
	if err != nil {
		return nil, err
	}

	if current.Generation != expectedGeneration {
		return nil, biz.ErrResourceVersionConflict
	}
	if err := uc.checkReferences(ctx, spec); err != nil {
		return nil, err
	}

	return uc.store.ReplaceSpec(ctx, current, spec)
}

// Delete 删除 Route，并在写入前检查当前可见的资源引用。
func (uc *Usecase) Delete(ctx context.Context, routeID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, routeID)
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

func (f ListFilter) matches(route resource.Route) bool {
	switch f.Type {
	case TypeAny:
	case TypeAPI:
		if route.Spec.AI != nil {
			return false
		}
	case TypeAI:
		if route.Spec.AI == nil {
			return false
		}
	default:
		return false
	}

	var searchText strings.Builder
	searchText.WriteString(route.Spec.DisplayName)
	searchText.WriteByte(' ')
	searchText.WriteString(route.Spec.Match.Path.Value)
	for _, hostname := range route.Spec.Hostnames {
		searchText.WriteByte(' ')
		searchText.WriteString(hostname)
	}
	status := biz.EnabledResourceStatus(
		route.Generation,
		route.Spec.Enabled,
		route.Status.Conditions,
	)
	return f.ResourceFilter.Match(searchText.String(), route.Spec.Enabled, status)
}
