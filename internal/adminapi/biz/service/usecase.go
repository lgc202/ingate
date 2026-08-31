// Package service 处理 Service 的业务规则和声明式资源协作。
package service

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

const (
	// TypeAny 不限制 Service 类型。
	TypeAny TypeFilter = iota
	// TypeHTTP 只返回普通 HTTP Service。
	TypeHTTP
	// TypeModel 只返回模型 Service。
	TypeModel
)

// TypeFilter 表达 Service 列表的类型筛选条件。
type TypeFilter uint8

// Store 定义 Service 管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Upstream], error)
	Get(ctx context.Context, serviceID string) (*resource.Upstream, error)
	Create(ctx context.Context, serviceID string, spec resource.UpstreamSpec) (*resource.Upstream, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.Upstream,
		spec resource.UpstreamSpec,
	) (*resource.Upstream, error)
	Delete(ctx context.Context, observed *resource.Upstream) error
}

// RouteLister 定义 Service 删除检查所需的 Route 分页能力。
type RouteLister interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Route], error)
}

// ListFilter 表达 Service 列表的筛选条件。
type ListFilter struct {
	biz.ResourceFilter
	Type TypeFilter
}

// ReplaceInput 描述 Service 替换操作及敏感字段的保留语义。
type ReplaceInput struct {
	ExpectedGeneration int64
	Spec               resource.UpstreamSpec
	PreserveAPIKey     bool
}

// Usecase 协调 Service 的校验、引用约束和持久化。
type Usecase struct {
	store  Store
	routes RouteLister
}

// NewUsecase 创建 Service 用例。
func NewUsecase(store Store, routes RouteLister) *Usecase {
	return &Usecase{store: store, routes: routes}
}

// List 返回满足筛选条件的 Service 列表。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter ListFilter,
) (biz.PageResult[resource.Upstream], error) {
	return biz.FilterPage(ctx, page, uc.store.ListPage, filter.matches)
}

// Get 返回指定 Service。
func (uc *Usecase) Get(ctx context.Context, serviceID string) (*resource.Upstream, error) {
	return uc.store.Get(ctx, serviceID)
}

// Create 创建 Service。
func (uc *Usecase) Create(ctx context.Context, spec resource.UpstreamSpec) (*resource.Upstream, error) {
	serviceID := uuid.NewString()
	return uc.store.Create(ctx, serviceID, spec)
}

// Replace 使用配置版本完整替换 Service 配置。
func (uc *Usecase) Replace(
	ctx context.Context,
	serviceID string,
	input ReplaceInput,
) (*resource.Upstream, error) {
	current, err := uc.store.Get(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	if current.Generation != input.ExpectedGeneration {
		return nil, biz.ErrResourceVersionConflict
	}
	replacement := input.Spec
	if input.PreserveAPIKey && current.Spec.Model != nil && replacement.Model != nil {
		model := *replacement.Model
		model.APIKey = current.Spec.Model.APIKey
		replacement.Model = &model
	}
	return uc.store.ReplaceSpec(ctx, current, replacement)
}

// Delete 删除 Service，并在写入前检查当前可见的 Route 引用。
func (uc *Usecase) Delete(ctx context.Context, serviceID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, serviceID)
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

func (f ListFilter) matches(service resource.Upstream) bool {
	isModel := service.Spec.Model != nil
	switch f.Type {
	case TypeAny:
	case TypeHTTP:
		if isModel {
			return false
		}
	case TypeModel:
		if !isModel {
			return false
		}
	default:
		return false
	}

	var searchText strings.Builder
	searchText.WriteString(service.Spec.DisplayName)
	for _, endpoint := range service.Spec.Endpoints {
		endpointKey := net.JoinHostPort(endpoint.Address, strconv.Itoa(endpoint.Port))
		searchText.WriteByte(' ')
		searchText.WriteString(endpointKey)
	}
	status := biz.ResourceStatusFromConditions(
		service.Generation,
		service.Status.Conditions,
	)
	return f.ResourceFilter.Match(searchText.String(), true, status)
}
