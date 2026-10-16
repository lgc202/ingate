package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1alpha1"
)

// ModelRoute 将写入交给 API Server，将查询交给 MySQL 投影。
type ModelRoute struct {
	client gatewayclient.ModelRoutesGetter
	store  *mysql.Store
}

// ModelRouteListParams 保存路由分页、展示名称包含匹配和 Gateway 精确筛选条件。
type ModelRouteListParams struct {
	Namespace   string
	PageSize    int32
	PageNum     int32
	DisplayName string
	GatewayName string
}

// ModelRouteInfo 保存管理查询及创建、更新响应所需的模型路由信息，不表示完整的 Kubernetes 资源。
type ModelRouteInfo struct {
	Namespace   string
	Name        string
	DisplayName string
	ParentRefs  []gatewayv1alpha1.ParentReference
	Rules       []gatewayv1alpha1.ModelRouteRule
	CreateTime  time.Time
	Version     string
}

// ModelRoutePage 保存一页产品查询视图及应用相同筛选条件的总数。
type ModelRoutePage struct {
	Items      []ModelRouteInfo
	TotalCount int64
}

// NewModelRoute 创建 ModelRoute Repository。
func NewModelRoute(client gatewayclient.ModelRoutesGetter, store *mysql.Store) *ModelRoute {
	return &ModelRoute{client: client, store: store}
}

// Create 创建 ModelRoute，并返回 API Server 保存后的业务状态。
func (r *ModelRoute) Create(
	ctx context.Context,
	route *gatewayv1alpha1.ModelRoute,
) (ModelRouteInfo, error) {
	created, err := r.client.ModelRoutes(route.Namespace).Create(ctx, route.DeepCopy(), metav1.CreateOptions{})
	if err != nil {
		return ModelRouteInfo{}, translateResourceError(err)
	}

	return modelRouteFromAPI(created), nil
}

// Get 从 MySQL 查询模型读取指定 ModelRoute。
func (r *ModelRoute) Get(
	ctx context.Context,
	namespace, name string,
) (ModelRouteInfo, error) {
	route, err := r.store.GetModelRoute(ctx, namespace, name)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return ModelRouteInfo{}, fmt.Errorf("%w: %w", ErrNotFound, err)
		}

		return ModelRouteInfo{}, err
	}

	return modelRouteFromMySQL(route), nil
}

// List 从 MySQL 查询模型按页码读取 ModelRoute。
func (r *ModelRoute) List(
	ctx context.Context,
	params ModelRouteListParams,
) (ModelRoutePage, error) {
	result, err := r.store.ListModelRoutes(ctx, mysql.ModelRouteListParams{
		Namespace:   params.Namespace,
		PageSize:    params.PageSize,
		PageNum:     params.PageNum,
		DisplayName: params.DisplayName,
		GatewayName: params.GatewayName,
	})
	if err != nil {
		return ModelRoutePage{}, err
	}

	routes := make([]ModelRouteInfo, len(result.Items))
	for i, route := range result.Items {
		routes[i] = modelRouteFromMySQL(route)
	}

	return ModelRoutePage{Items: routes, TotalCount: result.TotalCount}, nil
}

// Update 使用 API Server 当前对象核对版本，并替换 ModelRoute 的可编辑配置。
func (r *ModelRoute) Update(
	ctx context.Context,
	route *gatewayv1alpha1.ModelRoute,
) (ModelRouteInfo, error) {
	current, err := r.client.ModelRoutes(route.Namespace).Get(ctx, route.Name, metav1.GetOptions{})
	if err != nil {
		return ModelRouteInfo{}, translateResourceError(err)
	}

	if current.ResourceVersion != route.ResourceVersion {
		return ModelRouteInfo{}, ErrConflict
	}

	current.Spec = *route.Spec.DeepCopy()
	updated, err := r.client.ModelRoutes(route.Namespace).Update(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		return ModelRouteInfo{}, translateResourceError(err)
	}

	return modelRouteFromAPI(updated), nil
}

// Delete 删除指定 ModelRoute。
func (r *ModelRoute) Delete(ctx context.Context, namespace, name string) error {
	return translateResourceError(r.client.ModelRoutes(namespace).Delete(ctx, name, metav1.DeleteOptions{}))
}

func modelRouteFromAPI(route *gatewayv1alpha1.ModelRoute) ModelRouteInfo {
	spec := route.Spec.DeepCopy()

	return ModelRouteInfo{
		Namespace:   route.Namespace,
		Name:        route.Name,
		DisplayName: spec.DisplayName,
		ParentRefs:  spec.ParentRefs,
		Rules:       spec.Rules,
		CreateTime:  route.CreationTimestamp.Time,
		Version:     route.ResourceVersion,
	}
}

func modelRouteFromMySQL(route mysql.ModelRoute) ModelRouteInfo {
	result := ModelRouteInfo{
		Namespace:   route.Namespace,
		Name:        route.Name,
		DisplayName: route.DisplayName,
		CreateTime:  route.CreatedAt,
		Version:     route.Version,
	}

	for _, ref := range route.Parents {
		result.ParentRefs = append(result.ParentRefs, gatewayv1alpha1.ParentReference{
			Name:        ref.Name,
			SectionName: ref.SectionName,
		})
	}

	for _, rule := range route.Rules {
		target := gatewayv1alpha1.ModelRouteRule{Name: rule.Name}
		for _, match := range rule.Matches {
			condition := gatewayv1alpha1.ModelRouteMatch{}
			for _, header := range match.Headers {
				condition.Headers = append(condition.Headers, gatewayv1alpha1.HTTPHeaderMatch{
					Name:  header.Name,
					Type:  gatewayv1alpha1.HeaderMatchType(header.Type),
					Value: header.Value,
				})
			}

			target.Matches = append(target.Matches, condition)
		}

		for _, ref := range rule.BackendRefs {
			target.BackendRefs = append(target.BackendRefs, gatewayv1alpha1.AIBackendReference{
				Name:              ref.Name,
				ModelNameOverride: ref.ModelNameOverride,
			})
		}

		result.Rules = append(result.Rules, target)
	}

	return result
}
