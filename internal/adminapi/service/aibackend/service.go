// Package aibackend 实现模型服务资源的管理用例。
package aibackend

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/repository"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const defaultPageSize = 50

var (
	// ErrInvalidArgument 表示模型服务参数不合法。
	ErrInvalidArgument = errors.New("invalid AIBackend argument")
	// ErrAlreadyExists 表示生成的资源标识已存在。
	ErrAlreadyExists = errors.New("AIBackend already exists")
	// ErrNotFound 表示指定模型服务不存在。
	ErrNotFound = errors.New("AIBackend not found")
	// ErrConflict 表示模型服务版本已发生变化。
	ErrConflict = errors.New("AIBackend version conflict")
)

// CreateParams 保存创建 AIBackend 所需的参数。
type CreateParams struct {
	Namespace   string
	DisplayName string
	BackendRef  gatewayv1alpha1.BackendReference
	Schema      gatewayv1alpha1.APISchema
}

// UpdateParams 保存完整替换 AIBackend 可编辑配置所需的参数。
type UpdateParams struct {
	Namespace   string
	Name        string
	Version     string
	DisplayName string
	BackendRef  gatewayv1alpha1.BackendReference
	Schema      gatewayv1alpha1.APISchema
}

// ListParams 保存分页与展示名称包含匹配条件。
type ListParams struct {
	Namespace   string
	PageSize    int32
	PageNum     int32
	DisplayName string
}

// Service 实现 AIBackend 管理用例。
type Service struct {
	backends *repository.AIBackend
}

// New 创建 AIBackend Service。
func New(backends *repository.AIBackend) *Service {
	return &Service{backends: backends}
}

// Create 生成稳定资源标识并创建 AIBackend，不按展示名称去重。
func (s *Service) Create(
	ctx context.Context,
	params CreateParams,
) (repository.AIBackendInfo, error) {
	backend := &gatewayv1alpha1.AIBackend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: params.Namespace,
			Name:      "aibackend-" + strings.ToLower(rand.Text()),
		},
		Spec: gatewayv1alpha1.AIBackendSpec{
			DisplayName: params.DisplayName,
			BackendRef:  params.BackendRef,
			Schema:      params.Schema,
		},
	}

	created, err := s.backends.Create(ctx, backend)
	if err != nil {
		return repository.AIBackendInfo{}, translateRepositoryError(err)
	}

	return created, nil
}

// Get 读取指定 AIBackend。
func (s *Service) Get(
	ctx context.Context,
	namespace, name string,
) (repository.AIBackendInfo, error) {
	backend, err := s.backends.Get(ctx, namespace, name)
	if err != nil {
		return repository.AIBackendInfo{}, translateRepositoryError(err)
	}

	return backend, nil
}

// List 分页列出 AIBackend。
func (s *Service) List(
	ctx context.Context,
	params ListParams,
) (repository.AIBackendPage, error) {
	if params.PageSize == 0 {
		params.PageSize = defaultPageSize
	}
	if params.PageNum == 0 {
		params.PageNum = 1
	}

	page, err := s.backends.List(ctx, repository.ResourceListParams{
		Namespace:   params.Namespace,
		PageSize:    params.PageSize,
		PageNum:     params.PageNum,
		DisplayName: params.DisplayName,
	})
	if err != nil {
		return repository.AIBackendPage{}, translateRepositoryError(err)
	}

	return page, nil
}

// Update 完整替换 AIBackend 的可编辑配置，并检查调用方读取的版本。
func (s *Service) Update(
	ctx context.Context,
	params UpdateParams,
) (repository.AIBackendInfo, error) {
	backend := &gatewayv1alpha1.AIBackend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       params.Namespace,
			Name:            params.Name,
			ResourceVersion: params.Version,
		},
		Spec: gatewayv1alpha1.AIBackendSpec{
			DisplayName: params.DisplayName,
			BackendRef:  params.BackendRef,
			Schema:      params.Schema,
		},
	}

	updated, err := s.backends.Update(ctx, backend)
	if err != nil {
		return repository.AIBackendInfo{}, translateRepositoryError(err)
	}

	return updated, nil
}

// Delete 删除指定 AIBackend。
func (s *Service) Delete(ctx context.Context, namespace, name string) error {
	return translateRepositoryError(s.backends.Delete(ctx, namespace, name))
}

func translateRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrInvalid):
		return fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	case errors.Is(err, repository.ErrAlreadyExists):
		return fmt.Errorf("%w: %w", ErrAlreadyExists, err)
	case errors.Is(err, repository.ErrNotFound):
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case errors.Is(err, repository.ErrConflict):
		return fmt.Errorf("%w: %w", ErrConflict, err)
	default:
		return err
	}
}
