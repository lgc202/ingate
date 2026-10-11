package backend

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
	// ErrInvalidArgument 表示后端资源参数不合法。
	ErrInvalidArgument = errors.New("invalid backend argument")
	// ErrAlreadyExists 表示生成的资源标识已存在。
	ErrAlreadyExists = errors.New("backend already exists")
	// ErrNotFound 表示指定后端资源不存在。
	ErrNotFound = errors.New("backend not found")
	// ErrConflict 表示后端资源版本已发生变化。
	ErrConflict = errors.New("backend version conflict")
)

// CreateParams 保存创建 Backend 所需的参数。
type CreateParams struct {
	DisplayName string
	Endpoints   []gatewayv1alpha1.Endpoint
}

// UpdateParams 保存完整替换 Backend 可编辑配置所需的参数。
type UpdateParams struct {
	Name        string
	Version     string
	DisplayName string
	Endpoints   []gatewayv1alpha1.Endpoint
}

// ListParams 保存分页与展示名称包含匹配条件。
type ListParams struct {
	PageSize    int32
	PageNum     int32
	DisplayName string
}

// Service 实现 Backend 管理用例。
type Service struct {
	backends *repository.Backend
}

// New 创建 Backend Service。
func New(backends *repository.Backend) *Service {
	return &Service{backends: backends}
}

// Create 生成稳定资源标识并创建 Backend，不按展示名称去重。
func (s *Service) Create(
	ctx context.Context,
	params CreateParams,
) (repository.BackendInfo, error) {
	backend := &gatewayv1alpha1.Backend{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-" + strings.ToLower(rand.Text())},
		Spec: gatewayv1alpha1.BackendSpec{
			DisplayName: params.DisplayName,
			Endpoints:   params.Endpoints,
		},
	}

	created, err := s.backends.Create(ctx, backend)
	if err != nil {
		return repository.BackendInfo{}, translateRepositoryError(err)
	}

	return created, nil
}

// Get 读取指定 Backend。
func (s *Service) Get(
	ctx context.Context,
	name string,
) (repository.BackendInfo, error) {
	backend, err := s.backends.Get(ctx, name)
	if err != nil {
		return repository.BackendInfo{}, translateRepositoryError(err)
	}

	return backend, nil
}

// List 分页列出 Backend。
func (s *Service) List(
	ctx context.Context,
	params ListParams,
) (repository.BackendPage, error) {
	if params.PageSize == 0 {
		params.PageSize = defaultPageSize
	}
	if params.PageNum == 0 {
		params.PageNum = 1
	}

	page, err := s.backends.List(ctx, repository.ResourceListParams{
		PageSize:    params.PageSize,
		PageNum:     params.PageNum,
		DisplayName: params.DisplayName,
	})
	if err != nil {
		return repository.BackendPage{}, translateRepositoryError(err)
	}

	return page, nil
}

// Update 完整替换 Backend 的可编辑配置，并检查调用方读取的版本。
func (s *Service) Update(
	ctx context.Context,
	params UpdateParams,
) (repository.BackendInfo, error) {
	backend := &gatewayv1alpha1.Backend{
		ObjectMeta: metav1.ObjectMeta{
			Name:            params.Name,
			ResourceVersion: params.Version,
		},
		Spec: gatewayv1alpha1.BackendSpec{
			DisplayName: params.DisplayName,
			Endpoints:   params.Endpoints,
		},
	}

	updated, err := s.backends.Update(ctx, backend)
	if err != nil {
		return repository.BackendInfo{}, translateRepositoryError(err)
	}

	return updated, nil
}

// Delete 删除指定 Backend。
func (s *Service) Delete(ctx context.Context, name string) error {
	return translateRepositoryError(s.backends.Delete(ctx, name))
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
