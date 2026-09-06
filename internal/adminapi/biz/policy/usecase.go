package policy

import (
	"context"

	"github.com/google/uuid"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/query"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义策略管理用例依赖的持久化能力。
type Store[Policy, Spec any] interface {
	ListPage(ctx context.Context, page pagination.Request) (pagination.Result[Policy], error)
	Get(ctx context.Context, policyID string) (*Policy, error)
	Create(ctx context.Context, policyID string, spec Spec) (*Policy, error)
	ReplaceSpec(ctx context.Context, observed *Policy, spec Spec) (*Policy, error)
	Delete(ctx context.Context, observed *Policy) error
}

// ResourceAccessors 描述策略管理骨架如何读取具体资源和配置。
type ResourceAccessors[Policy, Spec any] struct {
	DisplayName        func(*Policy) string
	Enabled            func(*Policy) bool
	Generation         func(*Policy) int64
	TargetRefs         func(*Policy) []resource.PolicyTargetRef
	TargetRefsFromSpec func(Spec) []resource.PolicyTargetRef
	Status             func(*Policy) status.Status
}

// Usecase 实现所有声明式策略共享的查询、版本校验、目标解析和持久化流程。
type Usecase[Policy, Spec any] struct {
	store            Store[Policy, Spec]
	targets          *TargetResolver
	resources        ResourceAccessors[Policy, Spec]
	validateMutation func(context.Context, string, Spec) error
}

// NewUsecase 创建声明式策略的共享管理用例。
func NewUsecase[Policy, Spec any](
	store Store[Policy, Spec],
	targets *TargetResolver,
	resources ResourceAccessors[Policy, Spec],
	validateMutation func(context.Context, string, Spec) error,
) *Usecase[Policy, Spec] {
	return &Usecase[Policy, Spec]{
		store:            store,
		targets:          targets,
		resources:        resources,
		validateMutation: validateMutation,
	}
}

// List 返回满足筛选条件的一页策略及其目标展示名称。
func (uc *Usecase[Policy, Spec]) List(
	ctx context.Context,
	page pagination.Request,
	filter query.Filter,
) (Page[Policy], error) {
	result, err := query.FilterPage(ctx, page, uc.store.ListPage, func(item Policy) bool {
		return filter.Match(
			uc.resources.DisplayName(&item),
			uc.resources.Enabled(&item),
			uc.resources.Status(&item).State,
		)
	})
	if err != nil {
		return Page[Policy]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, uc.collectTargetRefs(result.Items))
	if err != nil {
		return Page[Policy]{}, err
	}
	return Page[Policy]{
		Items:       result.Items,
		TargetNames: targetNames,
		NextCursor:  result.NextCursor,
	}, nil
}

// Get 返回指定策略及其目标展示名称。
func (uc *Usecase[Policy, Spec]) Get(ctx context.Context, policyID string) (View[Policy], error) {
	item, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return View[Policy]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, uc.resources.TargetRefs(item))
	if err != nil {
		return View[Policy]{}, err
	}
	return View[Policy]{Policy: item, TargetNames: targetNames}, nil
}

// Create 校验策略变更和目标引用后创建资源。
func (uc *Usecase[Policy, Spec]) Create(ctx context.Context, spec Spec) (View[Policy], error) {
	if err := uc.validate(ctx, "", spec); err != nil {
		return View[Policy]{}, err
	}
	targetNames, err := uc.targets.Resolve(ctx, uc.resources.TargetRefsFromSpec(spec))
	if err != nil {
		return View[Policy]{}, err
	}
	item, err := uc.store.Create(ctx, uuid.NewString(), spec)
	if err != nil {
		return View[Policy]{}, err
	}
	return View[Policy]{Policy: item, TargetNames: targetNames}, nil
}

// Replace 使用配置版本完整替换策略。
func (uc *Usecase[Policy, Spec]) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec Spec,
) (View[Policy], error) {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return View[Policy]{}, err
	}
	if uc.resources.Generation(current) != expectedGeneration {
		return View[Policy]{}, adminv1.ErrorResourceVersionConflict("资源已被其他用户修改，请刷新后重试")
	}
	if err := uc.validate(ctx, policyID, spec); err != nil {
		return View[Policy]{}, err
	}
	targetNames, err := uc.targets.Resolve(ctx, uc.resources.TargetRefsFromSpec(spec))
	if err != nil {
		return View[Policy]{}, err
	}
	item, err := uc.store.ReplaceSpec(ctx, current, spec)
	if err != nil {
		return View[Policy]{}, err
	}
	return View[Policy]{Policy: item, TargetNames: targetNames}, nil
}

// Delete 使用配置版本删除策略。
func (uc *Usecase[Policy, Spec]) Delete(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
) error {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if uc.resources.Generation(current) != expectedGeneration {
		return adminv1.ErrorResourceVersionConflict("资源已被其他用户修改，请刷新后重试")
	}
	return uc.store.Delete(ctx, current)
}

func (uc *Usecase[Policy, Spec]) validate(ctx context.Context, excludedPolicyID string, spec Spec) error {
	if uc.validateMutation == nil {
		return nil
	}
	return uc.validateMutation(ctx, excludedPolicyID, spec)
}

func (uc *Usecase[Policy, Spec]) collectTargetRefs(items []Policy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for i := range items {
		refs = append(refs, uc.resources.TargetRefs(&items[i])...)
	}
	return refs
}
