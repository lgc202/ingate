package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1alpha1"
)

// MCPRoute 将写入交给 API Server，将查询交给 MySQL 投影。
type MCPRoute struct {
	client gatewayclient.MCPRouteInterface
	store  *mysql.Store
}

// MCPRouteListParams 保存路由分页、展示名称包含匹配和 Gateway 精确筛选条件。
type MCPRouteListParams struct {
	PageSize    int32
	PageNum     int32
	DisplayName string
	GatewayName string
}

// MCPRouteInfo 保存管理查询及创建、更新响应所需的 MCP 路由信息，不表示完整的 Kubernetes 资源。
type MCPRouteInfo struct {
	Name        string
	DisplayName string
	ParentRefs  []gatewayv1alpha1.ParentReference
	Path        *string
	BackendRefs []gatewayv1alpha1.MCPBackendReference
	CreateTime  time.Time
	Version     string
}

// MCPRoutePage 保存一页产品查询视图及应用相同筛选条件的总数。
type MCPRoutePage struct {
	Items      []MCPRouteInfo
	TotalCount int64
}

// NewMCPRoute 创建 MCPRoute Repository。
func NewMCPRoute(client gatewayclient.MCPRouteInterface, store *mysql.Store) *MCPRoute {
	return &MCPRoute{client: client, store: store}
}

// Create 创建 MCPRoute，并返回 API Server 保存后的业务状态。
func (r *MCPRoute) Create(
	ctx context.Context,
	route *gatewayv1alpha1.MCPRoute,
) (MCPRouteInfo, error) {
	created, err := r.client.Create(ctx, route.DeepCopy(), metav1.CreateOptions{})
	if err != nil {
		return MCPRouteInfo{}, translateResourceError(err)
	}

	return mcpRouteFromAPI(created), nil
}

// Get 从 MySQL 查询模型读取指定 MCPRoute。
func (r *MCPRoute) Get(
	ctx context.Context,
	name string,
) (MCPRouteInfo, error) {
	route, err := r.store.GetMCPRoute(ctx, name)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return MCPRouteInfo{}, fmt.Errorf("%w: %w", ErrNotFound, err)
		}
		return MCPRouteInfo{}, err
	}

	return mcpRouteFromMySQL(route), nil
}

// List 从 MySQL 查询模型按页码读取 MCPRoute。
func (r *MCPRoute) List(
	ctx context.Context,
	params MCPRouteListParams,
) (MCPRoutePage, error) {
	result, err := r.store.ListMCPRoutes(ctx, mysql.MCPRouteListParams{
		PageSize:    params.PageSize,
		PageNum:     params.PageNum,
		DisplayName: params.DisplayName,
		GatewayName: params.GatewayName,
	})
	if err != nil {
		return MCPRoutePage{}, err
	}

	routes := make([]MCPRouteInfo, len(result.Items))
	for i, route := range result.Items {
		routes[i] = mcpRouteFromMySQL(route)
	}

	return MCPRoutePage{Items: routes, TotalCount: result.TotalCount}, nil
}

// Update 使用 API Server 当前对象核对版本，并替换 MCPRoute 的可编辑配置。
func (r *MCPRoute) Update(
	ctx context.Context,
	route *gatewayv1alpha1.MCPRoute,
) (MCPRouteInfo, error) {
	current, err := r.client.Get(ctx, route.Name, metav1.GetOptions{})
	if err != nil {
		return MCPRouteInfo{}, translateResourceError(err)
	}

	if current.ResourceVersion != route.ResourceVersion {
		return MCPRouteInfo{}, ErrConflict
	}

	current.Spec = *route.Spec.DeepCopy()
	updated, err := r.client.Update(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		return MCPRouteInfo{}, translateResourceError(err)
	}

	return mcpRouteFromAPI(updated), nil
}

// Delete 删除指定 MCPRoute。
func (r *MCPRoute) Delete(ctx context.Context, name string) error {
	return translateResourceError(r.client.Delete(ctx, name, metav1.DeleteOptions{}))
}

func mcpRouteFromAPI(route *gatewayv1alpha1.MCPRoute) MCPRouteInfo {
	spec := route.Spec.DeepCopy()
	return MCPRouteInfo{
		Name:        route.Name,
		DisplayName: spec.DisplayName,
		ParentRefs:  spec.ParentRefs,
		Path:        spec.Path,
		BackendRefs: spec.BackendRefs,
		CreateTime:  route.CreationTimestamp.Time,
		Version:     route.ResourceVersion,
	}
}

func mcpRouteFromMySQL(route mysql.MCPRoute) MCPRouteInfo {
	result := MCPRouteInfo{
		Name:        route.Name,
		DisplayName: route.DisplayName,
		Path:        ptr.To(route.Path),
		BackendRefs: []gatewayv1alpha1.MCPBackendReference{{
			Name: route.BackendName,
			Path: ptr.To(route.BackendPath),
		}},
		CreateTime: route.CreatedAt,
		Version:    route.Version,
	}

	for _, ref := range route.Parents {
		result.ParentRefs = append(result.ParentRefs, gatewayv1alpha1.ParentReference{
			Name:        ref.Name,
			SectionName: ref.SectionName,
		})
	}

	return result
}
