package handler

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/errno"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	mcprouteservice "github.com/lgc202/ingate/internal/adminapi/service/mcproute"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

var (
	_ adminv1.MCPRouteServiceServer     = (*MCPRouteHandler)(nil)
	_ adminv1.MCPRouteServiceHTTPServer = (*MCPRouteHandler)(nil)
)

// MCPRouteHandler 实现 MCPRoute 的 gRPC 和 HTTP 协议入口。
type MCPRouteHandler struct {
	adminv1.UnimplementedMCPRouteServiceServer

	service *mcprouteservice.Service
}

// NewMCPRoute 创建 MCP 路由协议入口，复用注入的业务服务。
func NewMCPRoute(service *mcprouteservice.Service) *MCPRouteHandler {
	return &MCPRouteHandler{service: service}
}

// CreateMCPRoute 创建 MCPRoute。
func (h *MCPRouteHandler) CreateMCPRoute(
	ctx context.Context,
	req *adminv1.CreateMCPRouteRequest,
) (*adminv1.CreateMCPRouteReply, error) {
	created, err := h.service.Create(ctx, mcprouteservice.CreateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		ParentRefs:  mcpRouteParentsFromProto(req.GetParentRefs()),
		Path:        req.Path,
		BackendRefs: mcpRouteBackendRefsFromProto(req.GetBackendRefs()),
	})
	if err != nil {
		return nil, toMCPRouteError(err)
	}

	return &adminv1.CreateMCPRouteReply{Route: mcpRouteToProto(created)}, nil
}

// GetMCPRoute 读取指定 MCPRoute。
func (h *MCPRouteHandler) GetMCPRoute(
	ctx context.Context,
	req *adminv1.GetMCPRouteRequest,
) (*adminv1.GetMCPRouteReply, error) {
	route, err := h.service.Get(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, toMCPRouteError(err)
	}

	return &adminv1.GetMCPRouteReply{Route: mcpRouteToProto(route)}, nil
}

// ListMCPRoutes 分页列出 MCPRoute。
func (h *MCPRouteHandler) ListMCPRoutes(
	ctx context.Context,
	req *adminv1.ListMCPRoutesRequest,
) (*adminv1.ListMCPRoutesReply, error) {
	page, err := h.service.List(ctx, mcprouteservice.ListParams{
		Namespace:   req.GetNamespace(),
		PageSize:    req.GetPageSize(),
		PageNum:     req.GetPageNum(),
		DisplayName: req.GetDisplayName(),
		GatewayName: req.GetGatewayName(),
	})
	if err != nil {
		return nil, toMCPRouteError(err)
	}

	routes := make([]*adminv1.MCPRoute, len(page.Items))
	for i, route := range page.Items {
		routes[i] = mcpRouteToProto(route)
	}

	return &adminv1.ListMCPRoutesReply{Routes: routes, TotalCount: page.TotalCount}, nil
}

// UpdateMCPRoute 完整替换 MCPRoute 的可编辑配置。
func (h *MCPRouteHandler) UpdateMCPRoute(
	ctx context.Context,
	req *adminv1.UpdateMCPRouteRequest,
) (*adminv1.UpdateMCPRouteReply, error) {
	updated, err := h.service.Update(ctx, mcprouteservice.UpdateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		ParentRefs:  mcpRouteParentsFromProto(req.GetParentRefs()),
		Path:        req.Path,
		BackendRefs: mcpRouteBackendRefsFromProto(req.GetBackendRefs()),
	})
	if err != nil {
		return nil, toMCPRouteError(err)
	}

	return &adminv1.UpdateMCPRouteReply{Route: mcpRouteToProto(updated)}, nil
}

// DeleteMCPRoute 删除指定 MCPRoute。
func (h *MCPRouteHandler) DeleteMCPRoute(
	ctx context.Context,
	req *adminv1.DeleteMCPRouteRequest,
) (*adminv1.DeleteMCPRouteReply, error) {
	if err := h.service.Delete(ctx, req.GetNamespace(), req.GetName()); err != nil {
		return nil, toMCPRouteError(err)
	}

	return &adminv1.DeleteMCPRouteReply{}, nil
}

func mcpRouteParentsFromProto(refs []*adminv1.ParentReference) []gatewayv1alpha1.ParentReference {
	result := make([]gatewayv1alpha1.ParentReference, len(refs))
	for i, ref := range refs {
		result[i] = gatewayv1alpha1.ParentReference{Name: ref.GetName(), SectionName: ref.GetSectionName()}
	}

	return result
}

func mcpRouteBackendRefsFromProto(refs []*adminv1.MCPBackendReference) []gatewayv1alpha1.MCPBackendReference {
	result := make([]gatewayv1alpha1.MCPBackendReference, len(refs))
	for i, ref := range refs {
		result[i] = gatewayv1alpha1.MCPBackendReference{
			Name: ref.GetName(),
			Path: ref.Path,
		}
	}

	return result
}

func mcpRouteToProto(route repository.MCPRouteInfo) *adminv1.MCPRoute {
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

func toMCPRouteError(err error) error {
	switch {
	case errors.Is(err, mcprouteservice.ErrInvalidArgument):
		return errno.ErrMCPRouteInvalidArgument.WithCause(err)
	case errors.Is(err, mcprouteservice.ErrAlreadyExists):
		return errno.ErrMCPRouteAlreadyExists.WithCause(err)
	case errors.Is(err, mcprouteservice.ErrNotFound):
		return errno.ErrMCPRouteNotFound.WithCause(err)
	case errors.Is(err, mcprouteservice.ErrConflict):
		return errno.ErrMCPRouteConflict.WithCause(err)
	default:
		return errno.Fallback(err)
	}
}
