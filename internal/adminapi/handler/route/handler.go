package route

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/domain"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	reasonInvalidArgument = "ROUTE_INVALID_ARGUMENT"
	reasonAlreadyExists   = "ROUTE_ALREADY_EXISTS"
	reasonNotFound        = "ROUTE_NOT_FOUND"
	reasonConflict        = "ROUTE_CONFLICT"
)

var (
	_ adminv1.RouteServiceServer     = (*Handler)(nil)
	_ adminv1.RouteServiceHTTPServer = (*Handler)(nil)
)

// Handler 实现 Route 的 gRPC 和 HTTP 协议入口。
type Handler struct {
	adminv1.UnimplementedRouteServiceServer

	service *routeservice.Service
}

// New 创建 Route Handler。
func New(service *routeservice.Service) *Handler {
	return &Handler{service: service}
}

// CreateRoute 创建 Route。
func (h *Handler) CreateRoute(
	ctx context.Context,
	req *adminv1.CreateRouteRequest,
) (*adminv1.CreateRouteReply, error) {
	created, err := h.service.Create(ctx, routeservice.CreateParams{
		Name:         req.GetName(),
		GatewayName:  req.GetGatewayName(),
		ListenerName: req.GetListenerName(),
		Path:         pathFromProto(req.GetPath()),
		UpstreamName: req.GetUpstreamName(),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.CreateRouteReply{Route: routeToProto(created)}, nil
}

// GetRoute 读取指定 Route。
func (h *Handler) GetRoute(
	ctx context.Context,
	req *adminv1.GetRouteRequest,
) (*adminv1.GetRouteReply, error) {
	route, err := h.service.Get(ctx, req.GetName())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.GetRouteReply{Route: routeToProto(route)}, nil
}

// ListRoutes 分页列出 Route。
func (h *Handler) ListRoutes(
	ctx context.Context,
	req *adminv1.ListRoutesRequest,
) (*adminv1.ListRoutesReply, error) {
	page, err := h.service.List(ctx, req.GetPageSize(), req.GetPageNum())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	routes := make([]*adminv1.Route, len(page.Items))
	for i, route := range page.Items {
		routes[i] = routeToProto(route)
	}

	return &adminv1.ListRoutesReply{Routes: routes, TotalCount: page.TotalCount}, nil
}

// UpdateRoute 完整替换 Route 的可编辑配置。
func (h *Handler) UpdateRoute(
	ctx context.Context,
	req *adminv1.UpdateRouteRequest,
) (*adminv1.UpdateRouteReply, error) {
	updated, err := h.service.Update(ctx, routeservice.UpdateParams{
		Name:         req.GetName(),
		Version:      req.GetVersion(),
		GatewayName:  req.GetGatewayName(),
		ListenerName: req.GetListenerName(),
		Path:         pathFromProto(req.GetPath()),
		UpstreamName: req.GetUpstreamName(),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.UpdateRouteReply{Route: routeToProto(updated)}, nil
}

// DeleteRoute 删除指定 Route。
func (h *Handler) DeleteRoute(
	ctx context.Context,
	req *adminv1.DeleteRouteRequest,
) (*adminv1.DeleteRouteReply, error) {
	if err := h.service.Delete(ctx, req.GetName()); err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.DeleteRouteReply{}, nil
}

func pathFromProto(path *adminv1.PathMatch) domain.PathMatch {
	return domain.PathMatch{
		Type:  pathTypeFromProto(path.GetType()),
		Value: path.GetValue(),
	}
}

func pathTypeFromProto(pathType adminv1.PathMatchType) string {
	switch pathType {
	case adminv1.PathMatchType_PATH_MATCH_TYPE_EXACT:
		return "Exact"
	case adminv1.PathMatchType_PATH_MATCH_TYPE_PREFIX:
		return "PathPrefix"
	default:
		return ""
	}
}

func routeToProto(route domain.Route) *adminv1.Route {
	return &adminv1.Route{
		Name:         route.Name,
		GatewayName:  route.GatewayName,
		ListenerName: route.ListenerName,
		Path: &adminv1.PathMatch{
			Type:  pathTypeToProto(route.Path.Type),
			Value: route.Path.Value,
		},
		UpstreamName: route.UpstreamName,
		CreateTime:   timestamppb.New(route.CreateTime),
		Version:      route.Version,
	}
}

func pathTypeToProto(pathType string) adminv1.PathMatchType {
	switch pathType {
	case "Exact":
		return adminv1.PathMatchType_PATH_MATCH_TYPE_EXACT
	case "PathPrefix":
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
		return kratoserrors.Conflict(reasonAlreadyExists, "同名路由已存在").WithCause(err)
	case errors.Is(err, routeservice.ErrNotFound):
		return kratoserrors.NotFound(reasonNotFound, "路由不存在").WithCause(err)
	case errors.Is(err, routeservice.ErrConflict):
		return kratoserrors.Conflict(reasonConflict, "路由已被修改，请刷新后重试").WithCause(err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}
