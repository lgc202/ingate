// Package upstream 处理 Upstream 的业务规则和资源协作。
package upstream

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
	// TypeAny 不限制 Upstream 类型。
	TypeAny TypeFilter = iota
	// TypeHTTP 只返回普通 HTTP Upstream。
	TypeHTTP
	// TypeModel 只返回模型 Upstream。
	TypeModel
)

// TypeFilter 表达 Upstream 列表的类型筛选条件。
type TypeFilter uint8

// Store 定义 Upstream 管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Upstream], error)
	Get(ctx context.Context, upstreamID string) (*resource.Upstream, error)
	Create(ctx context.Context, upstreamID string, spec resource.UpstreamSpec) (*resource.Upstream, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.Upstream,
		spec resource.UpstreamSpec,
	) (*resource.Upstream, error)
	Delete(ctx context.Context, observed *resource.Upstream) error
}

// RouteLister 定义 Upstream 删除检查所需的 Route 分页能力。
type RouteLister interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Route], error)
}

// ListFilter 表达 Upstream 列表的筛选条件。
type ListFilter struct {
	biz.ResourceFilter
	Type TypeFilter
}

// ReplaceInput 描述 Upstream 替换操作及敏感字段的保留语义。
type ReplaceInput struct {
	ExpectedGeneration int64
	Spec               resource.UpstreamSpec
	PreserveAPIKey     bool
}

// Usecase 协调 Upstream 的校验、引用约束和持久化。
type Usecase struct {
	store  Store
	routes RouteLister
}

// NewUsecase 创建 Upstream 用例。
func NewUsecase(store Store, routes RouteLister) *Usecase {
	return &Usecase{store: store, routes: routes}
}

// List 返回满足筛选条件的 Upstream 列表。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter ListFilter,
) (biz.PageResult[resource.Upstream], error) {
	return biz.FilterPage(ctx, page, uc.store.ListPage, filter.matches)
}

// Get 返回指定 Upstream。
func (uc *Usecase) Get(ctx context.Context, upstreamID string) (*resource.Upstream, error) {
	return uc.store.Get(ctx, upstreamID)
}

// Create 创建 Upstream。
func (uc *Usecase) Create(ctx context.Context, spec resource.UpstreamSpec) (*resource.Upstream, error) {
	upstreamID := uuid.NewString()
	return uc.store.Create(ctx, upstreamID, spec)
}

// Replace 使用配置版本完整替换 Upstream 配置。
func (uc *Usecase) Replace(
	ctx context.Context,
	upstreamID string,
	input ReplaceInput,
) (*resource.Upstream, error) {
	current, err := uc.store.Get(ctx, upstreamID)
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

// Delete 删除 Upstream，并在写入前检查当前可见的 Route 引用。
func (uc *Usecase) Delete(ctx context.Context, upstreamID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, upstreamID)
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

func (f ListFilter) matches(upstream resource.Upstream) bool {
	isModel := upstream.Spec.Model != nil
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
	searchText.WriteString(upstream.Spec.DisplayName)
	for _, endpoint := range upstream.Spec.Endpoints {
		endpointKey := net.JoinHostPort(endpoint.Address, strconv.Itoa(endpoint.Port))
		searchText.WriteByte(' ')
		searchText.WriteString(endpointKey)
	}
	status := biz.ResourceStatusFromConditions(
		upstream.Generation,
		upstream.Status.Conditions,
	)
	return f.ResourceFilter.Match(searchText.String(), true, status)
}
