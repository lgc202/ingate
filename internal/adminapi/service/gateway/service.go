package gateway

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
	// ErrInvalidArgument 表示网关参数不合法。
	ErrInvalidArgument = errors.New("invalid gateway argument")
	// ErrAlreadyExists 表示生成的资源标识已存在。
	ErrAlreadyExists = errors.New("gateway already exists")
	// ErrNotFound 表示指定网关不存在。
	ErrNotFound = errors.New("gateway not found")
	// ErrConflict 表示网关版本已发生变化。
	ErrConflict = errors.New("gateway version conflict")
)

// CreateParams 保存创建 Gateway 所需的参数。
type CreateParams struct {
	DisplayName string
	Listeners   []gatewayv1alpha1.Listener
}

// UpdateParams 保存完整替换 Gateway 可编辑配置所需的参数。
type UpdateParams struct {
	Name        string
	Version     string
	DisplayName string
	Listeners   []gatewayv1alpha1.Listener
}

// ListParams 保存分页与展示名称包含匹配条件。
type ListParams struct {
	PageSize    int32
	PageNum     int32
	DisplayName string
}

// Service 实现 Gateway 管理用例。
type Service struct {
	gateways *repository.Gateway
}

// New 创建 Gateway Service。
func New(gateways *repository.Gateway) *Service {
	return &Service{gateways: gateways}
}

// Create 生成稳定资源标识并创建 Gateway，不按展示名称去重。
func (s *Service) Create(
	ctx context.Context,
	params CreateParams,
) (repository.GatewayInfo, error) {
	gateway := &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-" + strings.ToLower(rand.Text())},
		Spec: gatewayv1alpha1.GatewaySpec{
			DisplayName: params.DisplayName,
			Listeners:   params.Listeners,
		},
	}

	created, err := s.gateways.Create(ctx, gateway)
	if err != nil {
		return repository.GatewayInfo{}, translateRepositoryError(err)
	}

	return created, nil
}

// Get 读取指定 Gateway。
func (s *Service) Get(
	ctx context.Context,
	name string,
) (repository.GatewayInfo, error) {
	gateway, err := s.gateways.Get(ctx, name)
	if err != nil {
		return repository.GatewayInfo{}, translateRepositoryError(err)
	}

	return gateway, nil
}

// List 分页列出 Gateway。
func (s *Service) List(
	ctx context.Context,
	params ListParams,
) (repository.GatewayPage, error) {
	if params.PageSize == 0 {
		params.PageSize = defaultPageSize
	}
	if params.PageNum == 0 {
		params.PageNum = 1
	}

	page, err := s.gateways.List(ctx, repository.ResourceListParams{
		PageSize:    params.PageSize,
		PageNum:     params.PageNum,
		DisplayName: params.DisplayName,
	})
	if err != nil {
		return repository.GatewayPage{}, translateRepositoryError(err)
	}

	return page, nil
}

// Update 完整替换 Gateway 的可编辑配置，并检查调用方读取的版本。
func (s *Service) Update(
	ctx context.Context,
	params UpdateParams,
) (repository.GatewayInfo, error) {
	gateway := &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:            params.Name,
			ResourceVersion: params.Version,
		},
		Spec: gatewayv1alpha1.GatewaySpec{
			DisplayName: params.DisplayName,
			Listeners:   params.Listeners,
		},
	}

	updated, err := s.gateways.Update(ctx, gateway)
	if err != nil {
		return repository.GatewayInfo{}, translateRepositoryError(err)
	}

	return updated, nil
}

// Delete 删除指定 Gateway。
func (s *Service) Delete(ctx context.Context, name string) error {
	return translateRepositoryError(s.gateways.Delete(ctx, name))
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
