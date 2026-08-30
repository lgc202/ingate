// Package certificate 处理 Certificate 的业务规则和资源协作。
package certificate

import (
	"context"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义 Certificate 管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Certificate], error)
	Get(ctx context.Context, certificateID string) (*resource.Certificate, error)
	Create(
		ctx context.Context,
		certificateID string,
		spec resource.CertificateSpec,
	) (*resource.Certificate, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.Certificate,
		spec resource.CertificateSpec,
	) (*resource.Certificate, error)
	Delete(ctx context.Context, observed *resource.Certificate) error
}

// GatewayLister 定义 Certificate 删除检查所需的 Gateway 分页能力。
type GatewayLister interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Gateway], error)
}

// ReplaceInput 描述 Certificate 替换操作及密钥对的保留语义。
type ReplaceInput struct {
	ExpectedGeneration int64
	Spec               resource.CertificateSpec
	PreserveKeyPair    bool
}

// Usecase 协调 Certificate 的校验、引用约束和持久化。
type Usecase struct {
	store    Store
	gateways GatewayLister
}

// NewUsecase 创建 Certificate 用例。
func NewUsecase(store Store, gateways GatewayLister) *Usecase {
	return &Usecase{store: store, gateways: gateways}
}

// List 返回满足筛选条件的 Certificate 列表。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (biz.PageResult[resource.Certificate], error) {
	return biz.FilterPage(ctx, page, uc.store.ListPage, func(certificate resource.Certificate) bool {
		status := biz.ResourceStatusFromConditions(
			certificate.Generation,
			certificate.Status.Conditions,
		)
		return filter.Match(certificate.Spec.DisplayName, true, status)
	})
}

// Get 返回指定 Certificate。
func (uc *Usecase) Get(ctx context.Context, certificateID string) (*resource.Certificate, error) {
	return uc.store.Get(ctx, certificateID)
}

// Create 创建 Certificate。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.CertificateSpec,
) (*resource.Certificate, error) {
	certificateID := uuid.NewString()
	return uc.store.Create(ctx, certificateID, spec)
}

// Replace 使用配置版本完整替换 Certificate 配置。
func (uc *Usecase) Replace(
	ctx context.Context,
	certificateID string,
	input ReplaceInput,
) (*resource.Certificate, error) {
	current, err := uc.store.Get(ctx, certificateID)
	if err != nil {
		return nil, err
	}

	if current.Generation != input.ExpectedGeneration {
		return nil, biz.ErrResourceVersionConflict
	}
	replacement := input.Spec
	if input.PreserveKeyPair {
		replacement.CertificatePEM = current.Spec.CertificatePEM
		replacement.PrivateKeyPEM = current.Spec.PrivateKeyPEM
	}
	return uc.store.ReplaceSpec(ctx, current, replacement)
}

// Delete 删除 Certificate，并在写入前检查当前可见的 Gateway 引用。
func (uc *Usecase) Delete(ctx context.Context, certificateID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, certificateID)
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
