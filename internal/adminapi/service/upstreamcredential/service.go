package upstreamcredential

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	credentialstore "github.com/lgc202/ingate/internal/adminapi/store/upstreamcredential"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 UpstreamCredential 管理用例
type Service struct {
	store     *credentialstore.Store
	upstreams *upstreamstore.Store
}

// New 创建 UpstreamCredential service
func New(store *credentialstore.Store, upstreams *upstreamstore.Store) *Service {
	return &Service{store: store, upstreams: upstreams}
}

// List 查询 UpstreamCredential 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	credentials, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListResult{Credentials: credentials.Items}, nil
}

// Get 查询单个 UpstreamCredential
func (s *Service) Get(ctx context.Context, credentialID string) (*CredentialResult, error) {
	credential, err := s.store.Get(ctx, credentialID)
	if err != nil {
		return nil, err
	}
	return &CredentialResult{Credential: credential}, nil
}

// Create 创建 UpstreamCredential
func (s *Service) Create(ctx context.Context, params CreateParams) (string, error) {
	if err := s.validateNameUnique(ctx, params.Name, ""); err != nil {
		return "", err
	}
	credential := credentialResource(uuid.NewString(), "", params.CredentialParams)
	created, err := s.store.Create(ctx, credential)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 UpstreamCredential，省略 API Key 时保留当前密钥
func (s *Service) Update(ctx context.Context, credentialID string, params UpdateParams) error {
	current, err := s.store.Get(ctx, credentialID)
	if err != nil {
		return err
	}
	if params.Version == "" {
		return xerrors.NewUserError("访问凭据版本不能为空")
	}
	if params.Version != current.ResourceVersion {
		return xerrors.NewUserError("访问凭据已被更新，请刷新后重试")
	}
	if err := s.validateNameUnique(ctx, params.Name, credentialID); err != nil {
		return err
	}

	next := current.DeepCopy()
	next.Spec.DisplayName = params.Name
	next.Spec.Type = params.Type
	if params.APIKeyValue != "" {
		next.Spec.APIKey = &resource.APIKeyCredential{Value: params.APIKeyValue}
	}
	_, err = s.store.Update(ctx, next)
	return err
}

// Delete 删除 UpstreamCredential
func (s *Service) Delete(ctx context.Context, credentialID string) error {
	upstreams, err := s.upstreams.List(ctx)
	if err != nil {
		return err
	}
	for _, upstream := range upstreams.Items {
		if upstream.Spec.CredentialRef != credentialID {
			continue
		}
		name := upstream.Spec.DisplayName
		if name == "" {
			name = upstream.Name
		}
		return xerrors.NewUserError(fmt.Sprintf("访问凭据仍被服务 %q 使用", name))
	}
	return s.store.Delete(ctx, credentialID)
}

func (s *Service) validateNameUnique(ctx context.Context, name, excludeID string) error {
	credentials, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, credential := range credentials.Items {
		if credential.Name == excludeID {
			continue
		}
		if credential.Spec.DisplayName == name {
			return xerrors.NewUserError(fmt.Sprintf("访问凭据名称 %q 已存在", name))
		}
	}
	return nil
}

func credentialResource(id, version string, params CredentialParams) *resource.UpstreamCredential {
	return &resource.UpstreamCredential{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindUpstreamCredential),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            id,
			ResourceVersion: version,
		},
		Spec: resource.UpstreamCredentialSpec{
			DisplayName: params.Name,
			Type:        params.Type,
			APIKey:      &resource.APIKeyCredential{Value: params.APIKeyValue},
		},
	}
}
