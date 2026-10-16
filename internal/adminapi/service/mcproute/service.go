// Package mcproute 实现 MCP 路由资源的管理用例。
package mcproute

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
	// ErrInvalidArgument 表示路由参数不合法。
	ErrInvalidArgument = errors.New("invalid MCP route argument")
	// ErrAlreadyExists 表示生成的资源标识已存在。
	ErrAlreadyExists = errors.New("MCP route already exists")
	// ErrNotFound 表示指定路由不存在。
	ErrNotFound = errors.New("MCP route not found")
	// ErrConflict 表示路由版本已发生变化。
	ErrConflict = errors.New("MCP route version conflict")
)

// CreateParams 保存创建 MCPRoute 所需的参数。
type CreateParams struct {
	Namespace   string
	DisplayName string
	ParentRefs  []gatewayv1alpha1.ParentReference
	Path        *string
	BackendRefs []gatewayv1alpha1.MCPBackendReference
}

// UpdateParams 保存完整替换 MCPRoute 可编辑配置所需的参数。
type UpdateParams struct {
	Namespace   string
	Name        string
	Version     string
	DisplayName string
	ParentRefs  []gatewayv1alpha1.ParentReference
	Path        *string
	BackendRefs []gatewayv1alpha1.MCPBackendReference
}

// ListParams 保存分页、展示名称包含匹配和 Gateway 精确筛选条件。
type ListParams struct {
	Namespace   string
	PageSize    int32
	PageNum     int32
	DisplayName string
	GatewayName string
}

// Service 实现 MCPRoute 管理用例。
type Service struct {
	routes *repository.MCPRoute
}

// New 创建 MCPRoute Service。
func New(routes *repository.MCPRoute) *Service {
	return &Service{routes: routes}
}

// Create 生成稳定资源标识并创建 MCPRoute，不按展示名称去重。
func (s *Service) Create(
	ctx context.Context,
	params CreateParams,
) (repository.MCPRouteInfo, error) {
	route := &gatewayv1alpha1.MCPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: params.Namespace,
			Name:      "mcproute-" + strings.ToLower(rand.Text()),
		},
		Spec: gatewayv1alpha1.MCPRouteSpec{
			DisplayName: params.DisplayName,
			ParentRefs:  params.ParentRefs,
			Path:        params.Path,
			BackendRefs: params.BackendRefs,
		},
	}

	created, err := s.routes.Create(ctx, route)
	if err != nil {
		return repository.MCPRouteInfo{}, translateRepositoryError(err)
	}

	return created, nil
}

// Get 读取指定 MCPRoute。
func (s *Service) Get(
	ctx context.Context,
	namespace, name string,
) (repository.MCPRouteInfo, error) {
	route, err := s.routes.Get(ctx, namespace, name)
	if err != nil {
		return repository.MCPRouteInfo{}, translateRepositoryError(err)
	}

	return route, nil
}

// List 分页列出 MCPRoute。
func (s *Service) List(
	ctx context.Context,
	params ListParams,
) (repository.MCPRoutePage, error) {
	if params.PageSize == 0 {
		params.PageSize = defaultPageSize
	}

	if params.PageNum == 0 {
		params.PageNum = 1
	}

	page, err := s.routes.List(ctx, repository.MCPRouteListParams{
		Namespace:   params.Namespace,
		PageSize:    params.PageSize,
		PageNum:     params.PageNum,
		DisplayName: params.DisplayName,
		GatewayName: params.GatewayName,
	})
	if err != nil {
		return repository.MCPRoutePage{}, translateRepositoryError(err)
	}

	return page, nil
}

// Update 完整替换 MCPRoute 的可编辑配置，并检查调用方读取的版本。
func (s *Service) Update(
	ctx context.Context,
	params UpdateParams,
) (repository.MCPRouteInfo, error) {
	route := &gatewayv1alpha1.MCPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       params.Namespace,
			Name:            params.Name,
			ResourceVersion: params.Version,
		},
		Spec: gatewayv1alpha1.MCPRouteSpec{
			DisplayName: params.DisplayName,
			ParentRefs:  params.ParentRefs,
			Path:        params.Path,
			BackendRefs: params.BackendRefs,
		},
	}

	updated, err := s.routes.Update(ctx, route)
	if err != nil {
		return repository.MCPRouteInfo{}, translateRepositoryError(err)
	}

	return updated, nil
}

// Delete 删除指定 MCPRoute。
func (s *Service) Delete(ctx context.Context, namespace, name string) error {
	return translateRepositoryError(s.routes.Delete(ctx, namespace, name))
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
