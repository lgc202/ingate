// Package mcproute 适配 MCPRouteService 的 HTTP 和 gRPC 协议。
package mcproute

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/mcproute"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const (
	reasonInvalidArgument = "MCP_ROUTE_INVALID_ARGUMENT"
	reasonAlreadyExists   = "MCP_ROUTE_ALREADY_EXISTS"
	reasonNotFound        = "MCP_ROUTE_NOT_FOUND"
	reasonConflict        = "MCP_ROUTE_CONFLICT"
)

var (
	_ adminv1.MCPRouteServiceServer     = (*Handler)(nil)
	_ adminv1.MCPRouteServiceHTTPServer = (*Handler)(nil)
)

// Handler 实现 MCPRoute 的 gRPC 和 HTTP 协议入口。
type Handler struct {
	adminv1.UnimplementedMCPRouteServiceServer

	service *routeservice.Service
}

// New 创建 MCPRoute Handler。
func New(service *routeservice.Service) *Handler {
	return &Handler{service: service}
}

// CreateMCPRoute 创建 MCPRoute。
func (h *Handler) CreateMCPRoute(
	ctx context.Context,
	req *adminv1.CreateMCPRouteRequest,
) (*adminv1.CreateMCPRouteReply, error) {
	created, err := h.service.Create(ctx, routeservice.CreateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		ParentRefs:  parentsFromProto(req.GetParentRefs()),
		Path:        req.Path,
		BackendRefs: backendRefsFromProto(req.GetBackendRefs()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.CreateMCPRouteReply{Route: routeToProto(created)}, nil
}

// GetMCPRoute 读取指定 MCPRoute。
func (h *Handler) GetMCPRoute(
	ctx context.Context,
	req *adminv1.GetMCPRouteRequest,
) (*adminv1.GetMCPRouteReply, error) {
	route, err := h.service.Get(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.GetMCPRouteReply{Route: routeToProto(route)}, nil
}

// ListMCPRoutes 分页列出 MCPRoute。
func (h *Handler) ListMCPRoutes(
	ctx context.Context,
	req *adminv1.ListMCPRoutesRequest,
) (*adminv1.ListMCPRoutesReply, error) {
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

	routes := make([]*adminv1.MCPRoute, len(page.Items))
	for i, route := range page.Items {
		routes[i] = routeToProto(route)
	}

	return &adminv1.ListMCPRoutesReply{Routes: routes, TotalCount: page.TotalCount}, nil
}

// UpdateMCPRoute 完整替换 MCPRoute 的可编辑配置。
func (h *Handler) UpdateMCPRoute(
	ctx context.Context,
	req *adminv1.UpdateMCPRouteRequest,
) (*adminv1.UpdateMCPRouteReply, error) {
	updated, err := h.service.Update(ctx, routeservice.UpdateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		ParentRefs:  parentsFromProto(req.GetParentRefs()),
		Path:        req.Path,
		BackendRefs: backendRefsFromProto(req.GetBackendRefs()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.UpdateMCPRouteReply{Route: routeToProto(updated)}, nil
}

// DeleteMCPRoute 删除指定 MCPRoute。
func (h *Handler) DeleteMCPRoute(
	ctx context.Context,
	req *adminv1.DeleteMCPRouteRequest,
) (*adminv1.DeleteMCPRouteReply, error) {
	if err := h.service.Delete(ctx, req.GetNamespace(), req.GetName()); err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.DeleteMCPRouteReply{}, nil
}

func parentsFromProto(refs []*adminv1.ParentReference) []gatewayv1alpha1.ParentReference {
	result := make([]gatewayv1alpha1.ParentReference, len(refs))
	for i, ref := range refs {
		result[i] = gatewayv1alpha1.ParentReference{Name: ref.GetName(), SectionName: ref.GetSectionName()}
	}
	return result
}

func backendRefsFromProto(refs []*adminv1.MCPBackendReference) []gatewayv1alpha1.MCPBackendReference {
	result := make([]gatewayv1alpha1.MCPBackendReference, len(refs))
	for i, ref := range refs {
		result[i] = gatewayv1alpha1.MCPBackendReference{
			Name: ref.GetName(),
			Path: ref.Path,
		}
	}
	return result
}

func routeToProto(route repository.MCPRouteInfo) *adminv1.MCPRoute {
	result := &adminv1.MCPRoute{
		Namespace:   route.Namespace,
		Name:        route.Name,
		DisplayName: route.DisplayName,
		Path:        route.Path,
		CreateTime:  timestamppb.New(route.CreateTime),
		Version:     route.Version,
	}

	for _, ref := range route.ParentRefs {
		result.ParentRefs = append(result.ParentRefs, &adminv1.ParentReference{Name: ref.Name, SectionName: ref.SectionName})
	}
	for _, ref := range route.BackendRefs {
		result.BackendRefs = append(result.BackendRefs, &adminv1.MCPBackendReference{
			Name: ref.Name,
			Path: ref.Path,
		})
	}

	return result
}

func toTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, routeservice.ErrInvalidArgument):
		return kratoserrors.BadRequest(reasonInvalidArgument, "MCP 路由参数不合法").WithCause(err)
	case errors.Is(err, routeservice.ErrAlreadyExists):
		return kratoserrors.Conflict(reasonAlreadyExists, "资源标识冲突，请重新创建").WithCause(err)
	case errors.Is(err, routeservice.ErrNotFound):
		return kratoserrors.NotFound(reasonNotFound, "MCP 路由不存在").WithCause(err)
	case errors.Is(err, routeservice.ErrConflict):
		return kratoserrors.Conflict(reasonConflict, "MCP 路由已被修改，请刷新后重试").WithCause(err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}
