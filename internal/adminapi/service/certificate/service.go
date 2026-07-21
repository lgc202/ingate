// Package certificate 实现 Certificate 管理用例
package certificate

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	certificatestore "github.com/lgc202/ingate/internal/adminapi/store/certificate"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// Service 承载 Certificate 管理用例
type Service struct {
	store    *certificatestore.Store
	gateways *gatewaystore.Store
}

// New 创建 Certificate service
func New(store *certificatestore.Store, gateways *gatewaystore.Store) *Service {
	return &Service{store: store, gateways: gateways}
}

// List 查询 Certificate 列表
func (s *Service) List(ctx context.Context) ([]resource.Certificate, error) {
	certificates, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return certificates.Items, nil
}

// Get 查询单个 Certificate
func (s *Service) Get(ctx context.Context, certificateID string) (*resource.Certificate, error) {
	certificate, err := s.store.Get(ctx, certificateID)
	if err != nil {
		return nil, err
	}
	return certificate, nil
}

// Create 创建 Certificate
func (s *Service) Create(ctx context.Context, spec resource.CertificateSpec) (string, error) {
	if err := s.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	certificate := &resource.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindCertificate),
		},
		ObjectMeta: metav1.ObjectMeta{Name: uuid.NewString()},
		Spec:       spec,
	}
	created, err := s.store.Create(ctx, certificate)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 Certificate
func (s *Service) Update(ctx context.Context, certificateID, version string, spec resource.CertificateSpec) error {
	if version == "" {
		return xerrors.NewUserError("证书版本不能为空")
	}

	// Generation 只随配置变化，重试时重新读取对象以避开 Controller 更新 status 引起的写冲突
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.store.Get(ctx, certificateID)
		if err != nil {
			return err
		}
		if version != strconv.FormatInt(current.Generation, 10) {
			return xerrors.NewUserError(fmt.Sprintf("证书 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		if err := s.validateNameUnique(ctx, spec.DisplayName, certificateID); err != nil {
			return err
		}

		next := current.DeepCopy()
		nextSpec := spec
		if nextSpec.CertificatePEM == "" {
			nextSpec.CertificatePEM = current.Spec.CertificatePEM
			nextSpec.PrivateKeyPEM = current.Spec.PrivateKeyPEM
		}
		next.Spec = nextSpec
		_, err = s.store.Update(ctx, next)
		return err
	})
}

// Delete 删除 Certificate，仍被 Gateway 引用时拒绝删除
func (s *Service) Delete(ctx context.Context, certificateID string) error {
	gateways, err := s.gateways.List(ctx)
	if err != nil {
		return err
	}
	for _, gateway := range gateways.Items {
		for _, listener := range gateway.Spec.Listeners {
			if listener.CertificateRef == certificateID {
				return xerrors.NewUserError(fmt.Sprintf("证书仍被网关 %q 引用", gateway.Spec.DisplayName))
			}
		}
	}
	return s.store.Delete(ctx, certificateID)
}

func (s *Service) validateNameUnique(ctx context.Context, name, excludeID string) error {
	certificates, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, certificate := range certificates.Items {
		if certificate.Name == excludeID {
			continue
		}
		if certificate.Spec.DisplayName == name {
			return xerrors.NewUserError(fmt.Sprintf("证书名称 %q 已存在", name))
		}
	}
	return nil
}
