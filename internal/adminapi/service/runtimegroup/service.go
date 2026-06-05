package runtimegroup

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	runtimegroupstore "github.com/lgc202/ingate/internal/adminapi/store/runtimegroup"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 RuntimeGroup 管理用例
type Service struct {
	store *runtimegroupstore.Store
}

// New 创建 RuntimeGroup service
func New(store *runtimegroupstore.Store) *Service {
	return &Service{store: store}
}

// List 查询 RuntimeGroup 列表，第一阶段会确保内置 default 已持久化
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	if err := s.EnsureDefault(ctx); err != nil {
		return nil, err
	}

	runtimeGroups, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListResult{RuntimeGroups: runtimeGroups.Items}, nil
}

// ValidateEnabled 校验 Gateway 引用的 RuntimeGroup 存在且启用
func (s *Service) ValidateEnabled(ctx context.Context, id string) error {
	if id == "" {
		id = DefaultID
	}
	if err := s.EnsureDefault(ctx); err != nil {
		return err
	}

	runtimeGroup, err := s.store.Get(ctx, id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return xerrors.NewUserError(fmt.Sprintf("运行组 %q 不存在", id))
		}
		return err
	}
	if !runtimeGroup.Spec.Enabled {
		return xerrors.NewUserError(fmt.Sprintf("运行组 %q 已停用", displayNameOrID(runtimeGroup.Name, runtimeGroup.Spec.DisplayName)))
	}
	return nil
}

// EnsureDefault 确保第一阶段内置运行组被持久化，避免 Gateway 引用悬空
func (s *Service) EnsureDefault(ctx context.Context) error {
	_, err := s.store.Get(ctx, DefaultID)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	_, err = s.store.Create(ctx, defaultRuntimeGroup())
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func defaultRuntimeGroup() *resource.RuntimeGroup {
	return &resource.RuntimeGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindRuntimeGroup),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultID,
		},
		Spec: resource.RuntimeGroupSpec{
			DisplayName: defaultDisplayName,
			Description: defaultDescription,
			Enabled:     true,
			TargetRef: resource.TargetRef{
				Name: DefaultTargetID,
			},
		},
	}
}

func displayNameOrID(id, displayName string) string {
	if displayName != "" {
		return displayName
	}
	return id
}
