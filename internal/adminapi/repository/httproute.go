package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayvalidation "github.com/lgc202/ingate/pkg/apis/gateway/validation"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1alpha1"
)

// HTTPRoute 将写入交给 API Server，将查询交给 MySQL 投影。
type HTTPRoute struct {
	client gatewayclient.HTTPRoutesGetter
	store  *mysql.Store
}

// HTTPRouteListParams 保存路由分页、展示名称包含匹配和 Gateway 精确筛选条件。
type HTTPRouteListParams struct {
	Namespace   string
	PageSize    int32
	PageNum     int32
	DisplayName string
	GatewayName string
}

// HTTPRouteInfo 保存管理查询及创建、更新响应所需的 HTTP 路由信息，不表示完整的 Kubernetes 资源。
type HTTPRouteInfo struct {
	Namespace   string
	Name        string
	DisplayName string
	ParentRefs  []gatewayv1alpha1.ParentReference
	Rules       []gatewayv1alpha1.HTTPRouteRule
	CreateTime  time.Time
	Version     string
}

// HTTPRoutePage 保存一页产品查询视图及应用相同筛选条件的总数。
type HTTPRoutePage struct {
	Items      []HTTPRouteInfo
	TotalCount int64
}

// NewHTTPRoute 创建 HTTPRoute Repository。
func NewHTTPRoute(client gatewayclient.HTTPRoutesGetter, store *mysql.Store) *HTTPRoute {
	return &HTTPRoute{client: client, store: store}
}

// Create 创建 HTTPRoute，并返回 API Server 保存后的业务状态。
func (r *HTTPRoute) Create(
	ctx context.Context,
	route *gatewayv1alpha1.HTTPRoute,
) (HTTPRouteInfo, error) {
	candidate := route.DeepCopy()
	gatewayv1alpha1.SetDefaultsHTTPRoute(candidate)
	if err := validateRoute(candidate); err != nil {
		return HTTPRouteInfo{}, err
	}

	created, err := r.client.HTTPRoutes(route.Namespace).Create(ctx, candidate, metav1.CreateOptions{})
	if err != nil {
		return HTTPRouteInfo{}, translateResourceError(err)
	}

	return routeFromAPI(created), nil
}

// Get 从 MySQL 查询模型读取指定 HTTPRoute。
func (r *HTTPRoute) Get(
	ctx context.Context,
	namespace, name string,
) (HTTPRouteInfo, error) {
	route, err := r.store.GetHTTPRoute(ctx, namespace, name)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return HTTPRouteInfo{}, fmt.Errorf("%w: %w", ErrNotFound, err)
		}

		return HTTPRouteInfo{}, err
	}

	return routeFromMySQL(route), nil
}

// List 从 MySQL 查询模型按页码读取 HTTPRoute。
func (r *HTTPRoute) List(
	ctx context.Context,
	params HTTPRouteListParams,
) (HTTPRoutePage, error) {
	result, err := r.store.ListHTTPRoutes(ctx, mysql.HTTPRouteListParams{
		Namespace:   params.Namespace,
		PageSize:    params.PageSize,
		PageNum:     params.PageNum,
		DisplayName: params.DisplayName,
		GatewayName: params.GatewayName,
	})
	if err != nil {
		return HTTPRoutePage{}, err
	}

	routes := make([]HTTPRouteInfo, len(result.Items))
	for i, route := range result.Items {
		routes[i] = routeFromMySQL(route)
	}

	return HTTPRoutePage{Items: routes, TotalCount: result.TotalCount}, nil
}

// Update 使用 API Server 当前对象核对版本，并替换 HTTPRoute 的可编辑配置。
func (r *HTTPRoute) Update(
	ctx context.Context,
	route *gatewayv1alpha1.HTTPRoute,
) (HTTPRouteInfo, error) {
	current, err := r.client.HTTPRoutes(route.Namespace).Get(ctx, route.Name, metav1.GetOptions{})
	if err != nil {
		return HTTPRouteInfo{}, translateResourceError(err)
	}

	if current.ResourceVersion != route.ResourceVersion {
		return HTTPRouteInfo{}, ErrConflict
	}

	candidate := route.DeepCopy()
	gatewayv1alpha1.SetDefaultsHTTPRoute(candidate)
	if err := validateRoute(candidate); err != nil {
		return HTTPRouteInfo{}, err
	}

	current.Spec = candidate.Spec
	updated, err := r.client.HTTPRoutes(route.Namespace).Update(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		return HTTPRouteInfo{}, translateResourceError(err)
	}

	return routeFromAPI(updated), nil
}

// Delete 删除指定 HTTPRoute。
func (r *HTTPRoute) Delete(ctx context.Context, namespace, name string) error {
	return translateResourceError(r.client.HTTPRoutes(namespace).Delete(ctx, name, metav1.DeleteOptions{}))
}

func validateRoute(route *gatewayv1alpha1.HTTPRoute) error {
	var internal gatewayapi.HTTPRoute
	if err := gatewayv1alpha1.Convert_v1alpha1_HTTPRoute_To_gateway_HTTPRoute(route, &internal, nil); err != nil {
		return fmt.Errorf("convert HTTPRoute: %w", err)
	}

	if errs := gatewayvalidation.ValidateHTTPRoute(&internal); len(errs) > 0 {
		return fmt.Errorf("%w: %w", ErrInvalid, errs.ToAggregate())
	}

	return nil
}

func routeFromAPI(route *gatewayv1alpha1.HTTPRoute) HTTPRouteInfo {
	spec := route.DeepCopy().Spec

	return HTTPRouteInfo{
		Namespace:   route.Namespace,
		Name:        route.Name,
		DisplayName: spec.DisplayName,
		ParentRefs:  spec.ParentRefs,
		Rules:       spec.Rules,
		CreateTime:  route.CreationTimestamp.Time,
		Version:     route.ResourceVersion,
	}
}

func routeFromMySQL(route mysql.HTTPRoute) HTTPRouteInfo {
	result := HTTPRouteInfo{
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
		target := gatewayv1alpha1.HTTPRouteRule{Name: rule.Name}
		for _, match := range rule.Matches {
			target.Matches = append(target.Matches, gatewayv1alpha1.HTTPRouteMatch{
				Path: &gatewayv1alpha1.PathMatch{
					Type:  gatewayv1alpha1.PathMatchType(match.PathType),
					Value: match.PathValue,
				},
			})
		}

		for _, ref := range rule.BackendRefs {
			target.BackendRefs = append(target.BackendRefs, gatewayv1alpha1.BackendReference{Name: ref.Name})
		}

		result.Rules = append(result.Rules, target)
	}

	return result
}
