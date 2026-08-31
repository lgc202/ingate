// Package caller 处理 Caller、Route 授权和访问密钥生命周期。
package caller

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义 Caller 管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Caller], error)
	Get(ctx context.Context, callerID string) (*resource.Caller, error)
	Create(ctx context.Context, callerID string, spec resource.CallerSpec) (*resource.Caller, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.Caller,
		spec resource.CallerSpec,
	) (*resource.Caller, error)
	Delete(ctx context.Context, observed *resource.Caller) error
}

// RouteReader 定义 Caller 校验 Route 授权所需的批量读取能力。
type RouteReader interface {
	ListByIDs(ctx context.Context, routeIDs []string) (map[string]*resource.Route, error)
}

// TokenQuotaPolicyLister 定义 Caller 删除检查所需的 Token 额度策略分页能力。
type TokenQuotaPolicyLister interface {
	ListPage(
		ctx context.Context,
		page biz.PageRequest,
	) (biz.PageResult[resource.TokenQuotaPolicy], error)
}

// CreateInput 描述创建 Caller 并签发首个访问密钥所需的信息。
type CreateInput struct {
	Spec                 resource.CallerSpec
	AccessKeyDisplayName string
	AccessKeyExpiresAt   *time.Time
}

// Usecase 协调 Caller 权限、访问密钥和持久化。
type Usecase struct {
	store              Store
	routes             RouteReader
	tokenQuotaPolicies TokenQuotaPolicyLister
}

// NewUsecase 创建 Caller 用例。
func NewUsecase(
	store Store,
	routes RouteReader,
	tokenQuotaPolicies TokenQuotaPolicyLister,
) *Usecase {
	return &Usecase{
		store:              store,
		routes:             routes,
		tokenQuotaPolicies: tokenQuotaPolicies,
	}
}

// List 返回满足筛选条件的 Caller 列表。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (biz.PageResult[resource.Caller], error) {
	return biz.FilterPage(ctx, page, uc.store.ListPage, func(caller resource.Caller) bool {
		return filter.Match(caller.Spec.DisplayName, caller.Spec.Enabled, biz.ResourceStatus{})
	})
}

// Get 返回指定 Caller。
func (uc *Usecase) Get(ctx context.Context, callerID string) (*resource.Caller, error) {
	return uc.store.Get(ctx, callerID)
}

// Create 创建 Caller 并签发首个访问密钥。
func (uc *Usecase) Create(
	ctx context.Context,
	input CreateInput,
) (*resource.Caller, IssuedAccessKey, error) {
	if err := uc.checkAuthorizedRoutes(ctx, input.Spec.RouteRefs); err != nil {
		return nil, IssuedAccessKey{}, err
	}
	issuedAccessKey, err := newAccessKey(input.AccessKeyDisplayName, input.AccessKeyExpiresAt)
	if err != nil {
		return nil, IssuedAccessKey{}, err
	}

	spec := input.Spec
	spec.AccessKeys = []resource.AccessKey{issuedAccessKey.AccessKey}
	callerID := uuid.NewString()
	caller, err := uc.store.Create(ctx, callerID, spec)
	if err != nil {
		return nil, IssuedAccessKey{}, err
	}
	return caller, issuedAccessKey, nil
}

// Replace 使用配置版本完整替换 Caller 名称、启用状态和 Route 权限，并保留已有访问密钥。
func (uc *Usecase) Replace(
	ctx context.Context,
	callerID string,
	expectedGeneration int64,
	spec resource.CallerSpec,
) (*resource.Caller, error) {
	current, err := uc.store.Get(ctx, callerID)
	if err != nil {
		return nil, err
	}

	if current.Generation != expectedGeneration {
		return nil, biz.ErrResourceVersionConflict
	}
	if err := uc.checkAuthorizedRoutes(ctx, spec.RouteRefs); err != nil {
		return nil, err
	}

	spec.AccessKeys = slices.Clone(current.Spec.AccessKeys)
	return uc.store.ReplaceSpec(ctx, current, spec)
}

// Delete 删除 Caller；历史请求仍使用 Caller ID 保留归属。
func (uc *Usecase) Delete(ctx context.Context, callerID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, callerID)
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
