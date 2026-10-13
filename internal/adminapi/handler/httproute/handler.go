package httproute

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/httproute"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const (
	reasonInvalidArgument = "HTTP_ROUTE_INVALID_ARGUMENT"
	reasonAlreadyExists   = "HTTP_ROUTE_ALREADY_EXISTS"
	reasonNotFound        = "HTTP_ROUTE_NOT_FOUND"
	reasonConflict        = "HTTP_ROUTE_CONFLICT"
)

var (
	_ adminv1.HTTPRouteServiceServer     = (*Handler)(nil)
	_ adminv1.HTTPRouteServiceHTTPServer = (*Handler)(nil)
)

// Handler 实现 HTTPRoute 的 gRPC 和 HTTP 协议入口。
type Handler struct {
	adminv1.UnimplementedHTTPRouteServiceServer

	service *routeservice.Service
}

// New 创建 HTTPRoute Handler。
func New(service *routeservice.Service) *Handler {
	return &Handler{service: service}
}

// CreateHTTPRoute 创建 HTTPRoute。
func (h *Handler) CreateHTTPRoute(
	ctx context.Context,
	req *adminv1.CreateHTTPRouteRequest,
) (*adminv1.CreateHTTPRouteReply, error) {
	created, err := h.service.Create(ctx, routeservice.CreateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		ParentRefs:  parentsFromProto(req.GetParentRefs()),
		Rules:       rulesFromProto(req.GetRules()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.CreateHTTPRouteReply{Route: routeToProto(created)}, nil
}

// GetHTTPRoute 读取指定 HTTPRoute。
func (h *Handler) GetHTTPRoute(
	ctx context.Context,
	req *adminv1.GetHTTPRouteRequest,
) (*adminv1.GetHTTPRouteReply, error) {
	route, err := h.service.Get(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.GetHTTPRouteReply{Route: routeToProto(route)}, nil
}

// ListHTTPRoutes 分页列出 HTTPRoute。
func (h *Handler) ListHTTPRoutes(
	ctx context.Context,
	req *adminv1.ListHTTPRoutesRequest,
) (*adminv1.ListHTTPRoutesReply, error) {
	page, err := h.service.List(ctx, routeservice.ListParams{
		Namespace:   req.GetNamespace(),
		PageSize:    req.GetPageSize(),
		PageNum:     req.GetPageNum(),
		DisplayName: req.GetDisplayName(),
		GatewayName: req.GetGatewayName(),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	routes := make([]*adminv1.HTTPRoute, len(page.Items))
	for i, route := range page.Items {
		routes[i] = routeToProto(route)
	}

	return &adminv1.ListHTTPRoutesReply{Routes: routes, TotalCount: page.TotalCount}, nil
}

// UpdateHTTPRoute 完整替换 HTTPRoute 的可编辑配置。
func (h *Handler) UpdateHTTPRoute(
	ctx context.Context,
	req *adminv1.UpdateHTTPRouteRequest,
) (*adminv1.UpdateHTTPRouteReply, error) {
	updated, err := h.service.Update(ctx, routeservice.UpdateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		ParentRefs:  parentsFromProto(req.GetParentRefs()),
		Rules:       rulesFromProto(req.GetRules()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.UpdateHTTPRouteReply{Route: routeToProto(updated)}, nil
}

// DeleteHTTPRoute 删除指定 HTTPRoute。
func (h *Handler) DeleteHTTPRoute(
	ctx context.Context,
	req *adminv1.DeleteHTTPRouteRequest,
) (*adminv1.DeleteHTTPRouteReply, error) {
	if err := h.service.Delete(ctx, req.GetNamespace(), req.GetName()); err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.DeleteHTTPRouteReply{}, nil
}

func parentsFromProto(refs []*adminv1.ParentReference) []gatewayv1alpha1.ParentReference {
	result := make([]gatewayv1alpha1.ParentReference, len(refs))
	for i, ref := range refs {
		result[i] = gatewayv1alpha1.ParentReference{Name: ref.GetName(), SectionName: ref.GetSectionName()}
	}
	return result
}

func rulesFromProto(rules []*adminv1.HTTPRouteRule) []gatewayv1alpha1.HTTPRouteRule {
	result := make([]gatewayv1alpha1.HTTPRouteRule, len(rules))
	for i, rule := range rules {
		target := &result[i]
		target.Name = rule.GetName()
		for _, match := range rule.GetMatches() {
			var path *gatewayv1alpha1.PathMatch
			if match.GetPath() != nil {
				value := pathFromProto(match.GetPath())
				path = &value
			}
			target.Matches = append(target.Matches, gatewayv1alpha1.HTTPRouteMatch{Path: path})
		}
		for _, ref := range rule.GetBackendRefs() {
			target.BackendRefs = append(target.BackendRefs, gatewayv1alpha1.BackendReference{Name: ref.GetName()})
		}
	}
	return result
}

func pathFromProto(path *adminv1.PathMatch) gatewayv1alpha1.PathMatch {
	return gatewayv1alpha1.PathMatch{
		Type:  pathTypeFromProto(path.GetType()),
		Value: path.GetValue(),
	}
}

func pathTypeFromProto(pathType adminv1.PathMatchType) gatewayv1alpha1.PathMatchType {
	switch pathType {
	case adminv1.PathMatchType_PATH_MATCH_TYPE_EXACT:
		return gatewayv1alpha1.PathMatchExact
	case adminv1.PathMatchType_PATH_MATCH_TYPE_PREFIX:
		return gatewayv1alpha1.PathMatchPrefix
	default:
		return ""
	}
}

func routeToProto(route repository.HTTPRouteInfo) *adminv1.HTTPRoute {
	result := &adminv1.HTTPRoute{
		Namespace:   route.Namespace,
		Name:        route.Name,
		DisplayName: route.DisplayName,
		CreateTime:  timestamppb.New(route.CreateTime),
		Version:     route.Version,
	}

	for _, ref := range route.ParentRefs {
		result.ParentRefs = append(result.ParentRefs, &adminv1.ParentReference{Name: ref.Name, SectionName: ref.SectionName})
	}
	for _, rule := range route.Rules {
		target := &adminv1.HTTPRouteRule{Name: rule.Name}
		for _, match := range rule.Matches {
			var path *adminv1.PathMatch
			if match.Path != nil {
				path = &adminv1.PathMatch{Type: pathTypeToProto(match.Path.Type), Value: match.Path.Value}
			}
			target.Matches = append(target.Matches, &adminv1.HTTPRouteMatch{Path: path})
		}
		for _, ref := range rule.BackendRefs {
			target.BackendRefs = append(target.BackendRefs, &adminv1.BackendReference{Name: ref.Name})
		}
		result.Rules = append(result.Rules, target)
	}

	return result
}

func pathTypeToProto(pathType gatewayv1alpha1.PathMatchType) adminv1.PathMatchType {
	switch pathType {
	case gatewayv1alpha1.PathMatchExact:
		return adminv1.PathMatchType_PATH_MATCH_TYPE_EXACT
	case gatewayv1alpha1.PathMatchPrefix:
		return adminv1.PathMatchType_PATH_MATCH_TYPE_PREFIX
	default:
		return adminv1.PathMatchType_PATH_MATCH_TYPE_UNSPECIFIED
	}
}

func toTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, routeservice.ErrInvalidArgument):
		return kratoserrors.BadRequest(reasonInvalidArgument, "路由参数不合法").WithCause(err)
	case errors.Is(err, routeservice.ErrAlreadyExists):
		return kratoserrors.Conflict(reasonAlreadyExists, "资源标识冲突，请重新创建").WithCause(err)
	case errors.Is(err, routeservice.ErrNotFound):
		return kratoserrors.NotFound(reasonNotFound, "路由不存在").WithCause(err)
	case errors.Is(err, routeservice.ErrConflict):
		return kratoserrors.Conflict(reasonConflict, "路由已被修改，请刷新后重试").WithCause(err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}
