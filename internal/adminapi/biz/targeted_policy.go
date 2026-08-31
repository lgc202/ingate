package biz

import (
	"context"

	"github.com/google/uuid"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// PolicyStore 定义目标策略共享的持久化能力。
type PolicyStore[P, S any] interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[P], error)
	Get(ctx context.Context, policyID string) (*P, error)
	Create(ctx context.Context, policyID string, spec S) (*P, error)
	ReplaceSpec(ctx context.Context, observed *P, spec S) (*P, error)
	Delete(ctx context.Context, observed *P) error
}

// PolicyAttributes 是目标策略共享流程所需的领域属性。
type PolicyAttributes struct {
	Generation  int64
	DisplayName string
	Enabled     bool
	TargetRefs  []resource.PolicyTargetRef
	Status      ResourceStatus
}

// PolicyPage 保存一页目标策略及其目标展示名称。
type PolicyPage[P any] struct {
	Items       []P
	TargetNames PolicyTargetNames
	NextCursor  string
}

// PolicyView 保存单条目标策略及其目标展示名称。
type PolicyView[P any] struct {
	Policy      *P
	TargetNames PolicyTargetNames
}

// PolicyUsecase 实现目标策略共享的查询、引用校验和资源版本控制。
type PolicyUsecase[P, S any] struct {
	store        PolicyStore[P, S]
	targets      *PolicyTargetResolver
	attributesOf func(*P) PolicyAttributes
	targetsOf    func(S) []resource.PolicyTargetRef
}

// NewPolicyUsecase 创建目标策略的共享用例。
func NewPolicyUsecase[P, S any](
	store PolicyStore[P, S],
	targets *PolicyTargetResolver,
	attributesOf func(*P) PolicyAttributes,
	targetsOf func(S) []resource.PolicyTargetRef,
) *PolicyUsecase[P, S] {
	return &PolicyUsecase[P, S]{
		store:        store,
		targets:      targets,
		attributesOf: attributesOf,
		targetsOf:    targetsOf,
	}
}

// List 返回满足筛选条件的目标策略。
func (uc *PolicyUsecase[P, S]) List(
	ctx context.Context,
	page PageRequest,
	filter ResourceFilter,
) (PolicyPage[P], error) {
	result, err := FilterPage(ctx, page, uc.store.ListPage, func(policy P) bool {
		attributes := uc.attributesOf(&policy)
		return filter.Match(attributes.DisplayName, attributes.Enabled, attributes.Status)
	})
	if err != nil {
		return PolicyPage[P]{}, err
	}

	targetNames, err := uc.targets.DisplayNames(
		ctx,
		collectPolicyTargetRefs(result.Items, uc.attributesOf),
	)
	if err != nil {
		return PolicyPage[P]{}, err
	}
	return PolicyPage[P]{
		Items:       result.Items,
		TargetNames: targetNames,
		NextCursor:  result.NextCursor,
	}, nil
}

// Get 返回指定目标策略。
func (uc *PolicyUsecase[P, S]) Get(ctx context.Context, policyID string) (PolicyView[P], error) {
	policy, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return PolicyView[P]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, uc.attributesOf(policy).TargetRefs)
	if err != nil {
		return PolicyView[P]{}, err
	}
	return PolicyView[P]{Policy: policy, TargetNames: targetNames}, nil
}

// Create 校验目标引用并创建目标策略。
func (uc *PolicyUsecase[P, S]) Create(ctx context.Context, spec S) (PolicyView[P], error) {
	targetNames, err := uc.targets.Resolve(ctx, uc.targetsOf(spec))
	if err != nil {
		return PolicyView[P]{}, err
	}

	policy, err := uc.store.Create(ctx, uuid.NewString(), spec)
	if err != nil {
		return PolicyView[P]{}, err
	}
	return PolicyView[P]{Policy: policy, TargetNames: targetNames}, nil
}

// Replace 校验资源版本和目标引用后完整替换目标策略。
func (uc *PolicyUsecase[P, S]) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec S,
) (PolicyView[P], error) {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return PolicyView[P]{}, err
	}
	if uc.attributesOf(current).Generation != expectedGeneration {
		return PolicyView[P]{}, ErrResourceVersionConflict
	}
	return uc.ReplaceObserved(ctx, current, spec)
}

// ReplaceObserved 在调用方完成资源版本和领域规则校验后替换目标策略。
func (uc *PolicyUsecase[P, S]) ReplaceObserved(
	ctx context.Context,
	observed *P,
	spec S,
) (PolicyView[P], error) {
	targetNames, err := uc.targets.Resolve(ctx, uc.targetsOf(spec))
	if err != nil {
		return PolicyView[P]{}, err
	}
	policy, err := uc.store.ReplaceSpec(ctx, observed, spec)
	if err != nil {
		return PolicyView[P]{}, err
	}
	return PolicyView[P]{Policy: policy, TargetNames: targetNames}, nil
}

// Delete 校验资源版本后删除目标策略。
func (uc *PolicyUsecase[P, S]) Delete(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
) error {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if uc.attributesOf(current).Generation != expectedGeneration {
		return ErrResourceVersionConflict
	}
	return uc.store.Delete(ctx, current)
}

func collectPolicyTargetRefs[P any](
	policies []P,
	attributesOf func(*P) PolicyAttributes,
) []resource.PolicyTargetRef {
	targetCount := 0
	for i := range policies {
		targetCount += len(attributesOf(&policies[i]).TargetRefs)
	}

	refs := make([]resource.PolicyTargetRef, 0, targetCount)
	for i := range policies {
		refs = append(refs, attributesOf(&policies[i]).TargetRefs...)
	}
	return refs
}
