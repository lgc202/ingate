// Package modelroute 适配 ModelRouteService 的 HTTP 和 gRPC 协议。
package modelroute

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/modelroute"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const (
	reasonInvalidArgument = "MODEL_ROUTE_INVALID_ARGUMENT"
	reasonAlreadyExists   = "MODEL_ROUTE_ALREADY_EXISTS"
	reasonNotFound        = "MODEL_ROUTE_NOT_FOUND"
	reasonConflict        = "MODEL_ROUTE_CONFLICT"
)

var (
	_ adminv1.ModelRouteServiceServer     = (*Handler)(nil)
	_ adminv1.ModelRouteServiceHTTPServer = (*Handler)(nil)
)

// Handler 实现 ModelRoute 的 gRPC 和 HTTP 协议入口。
type Handler struct {
	adminv1.UnimplementedModelRouteServiceServer

	service *routeservice.Service
}

// New 创建 ModelRoute Handler。
func New(service *routeservice.Service) *Handler {
	return &Handler{service: service}
}

// CreateModelRoute 创建 ModelRoute。
func (h *Handler) CreateModelRoute(
	ctx context.Context,
	req *adminv1.CreateModelRouteRequest,
) (*adminv1.CreateModelRouteReply, error) {
	created, err := h.service.Create(ctx, routeservice.CreateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		ParentRefs:  parentsFromProto(req.GetParentRefs()),
		Rules:       rulesFromProto(req.GetRules()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.CreateModelRouteReply{Route: routeToProto(created)}, nil
}

// GetModelRoute 读取指定 ModelRoute。
func (h *Handler) GetModelRoute(
	ctx context.Context,
	req *adminv1.GetModelRouteRequest,
) (*adminv1.GetModelRouteReply, error) {
	route, err := h.service.Get(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.GetModelRouteReply{Route: routeToProto(route)}, nil
}

// ListModelRoutes 分页列出 ModelRoute。
func (h *Handler) ListModelRoutes(
	ctx context.Context,
	req *adminv1.ListModelRoutesRequest,
) (*adminv1.ListModelRoutesReply, error) {
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

	routes := make([]*adminv1.ModelRoute, len(page.Items))
	for i, route := range page.Items {
		routes[i] = routeToProto(route)
	}

	return &adminv1.ListModelRoutesReply{Routes: routes, TotalCount: page.TotalCount}, nil
}

// UpdateModelRoute 完整替换 ModelRoute 的可编辑配置。
func (h *Handler) UpdateModelRoute(
	ctx context.Context,
	req *adminv1.UpdateModelRouteRequest,
) (*adminv1.UpdateModelRouteReply, error) {
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

	return &adminv1.UpdateModelRouteReply{Route: routeToProto(updated)}, nil
}

// DeleteModelRoute 删除指定 ModelRoute。
func (h *Handler) DeleteModelRoute(
	ctx context.Context,
	req *adminv1.DeleteModelRouteRequest,
) (*adminv1.DeleteModelRouteReply, error) {
	if err := h.service.Delete(ctx, req.GetNamespace(), req.GetName()); err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.DeleteModelRouteReply{}, nil
}

func parentsFromProto(refs []*adminv1.ParentReference) []gatewayv1alpha1.ParentReference {
	result := make([]gatewayv1alpha1.ParentReference, len(refs))
	for i, ref := range refs {
		result[i] = gatewayv1alpha1.ParentReference{Name: ref.GetName(), SectionName: ref.GetSectionName()}
	}
	return result
}

func rulesFromProto(rules []*adminv1.ModelRouteRule) []gatewayv1alpha1.ModelRouteRule {
	result := make([]gatewayv1alpha1.ModelRouteRule, len(rules))
	for i, rule := range rules {
		target := &result[i]
		target.Name = rule.GetName()
		for _, match := range rule.GetMatches() {
			condition := gatewayv1alpha1.ModelRouteMatch{}
			for _, header := range match.GetHeaders() {
				condition.Headers = append(condition.Headers, gatewayv1alpha1.HTTPHeaderMatch{
					Name:  header.GetName(),
					Type:  gatewayv1alpha1.HeaderMatchType(header.GetType()),
					Value: header.GetValue(),
				})
			}
			target.Matches = append(target.Matches, condition)
		}
		for _, ref := range rule.GetBackendRefs() {
			target.BackendRefs = append(target.BackendRefs, gatewayv1alpha1.AIBackendReference{
				Name:              ref.GetName(),
				ModelNameOverride: ref.GetModelNameOverride(),
			})
		}
	}
	return result
}

func routeToProto(route repository.ModelRouteInfo) *adminv1.ModelRoute {
	result := &adminv1.ModelRoute{
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
		target := &adminv1.ModelRouteRule{Name: rule.Name}
		for _, match := range rule.Matches {
			condition := &adminv1.ModelRouteMatch{}
			for _, header := range match.Headers {
				condition.Headers = append(condition.Headers, &adminv1.HTTPHeaderMatch{
					Name:  header.Name,
					Type:  string(header.Type),
					Value: header.Value,
				})
			}
			target.Matches = append(target.Matches, condition)
		}
		for _, ref := range rule.BackendRefs {
			target.BackendRefs = append(target.BackendRefs, &adminv1.AIBackendReference{
				Name:              ref.Name,
				ModelNameOverride: ref.ModelNameOverride,
			})
		}
		result.Rules = append(result.Rules, target)
	}

	return result
}

func toTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, routeservice.ErrInvalidArgument):
		return kratoserrors.BadRequest(reasonInvalidArgument, "模型路由参数不合法").WithCause(err)
	case errors.Is(err, routeservice.ErrAlreadyExists):
		return kratoserrors.Conflict(reasonAlreadyExists, "资源标识冲突，请重新创建").WithCause(err)
	case errors.Is(err, routeservice.ErrNotFound):
		return kratoserrors.NotFound(reasonNotFound, "模型路由不存在").WithCause(err)
	case errors.Is(err, routeservice.ErrConflict):
		return kratoserrors.Conflict(reasonConflict, "模型路由已被修改，请刷新后重试").WithCause(err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}
