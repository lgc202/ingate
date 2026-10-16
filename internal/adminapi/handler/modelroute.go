package handler

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/errno"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	modelrouteservice "github.com/lgc202/ingate/internal/adminapi/service/modelroute"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

var (
	_ adminv1.ModelRouteServiceServer     = (*ModelRouteHandler)(nil)
	_ adminv1.ModelRouteServiceHTTPServer = (*ModelRouteHandler)(nil)
)

// ModelRouteHandler 实现 ModelRoute 的 gRPC 和 HTTP 协议入口。
type ModelRouteHandler struct {
	adminv1.UnimplementedModelRouteServiceServer

	service *modelrouteservice.Service
}

// NewModelRoute 创建模型路由协议入口，复用注入的业务服务。
func NewModelRoute(service *modelrouteservice.Service) *ModelRouteHandler {
	return &ModelRouteHandler{service: service}
}

// CreateModelRoute 创建 ModelRoute。
func (h *ModelRouteHandler) CreateModelRoute(
	ctx context.Context,
	req *adminv1.CreateModelRouteRequest,
) (*adminv1.CreateModelRouteReply, error) {
	created, err := h.service.Create(ctx, modelrouteservice.CreateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		ParentRefs:  modelRouteParentsFromProto(req.GetParentRefs()),
		Rules:       modelRouteRulesFromProto(req.GetRules()),
	})
	if err != nil {
		return nil, toModelRouteError(err)
	}

	return &adminv1.CreateModelRouteReply{Route: modelRouteToProto(created)}, nil
}

// GetModelRoute 读取指定 ModelRoute。
func (h *ModelRouteHandler) GetModelRoute(
	ctx context.Context,
	req *adminv1.GetModelRouteRequest,
) (*adminv1.GetModelRouteReply, error) {
	route, err := h.service.Get(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, toModelRouteError(err)
	}

	return &adminv1.GetModelRouteReply{Route: modelRouteToProto(route)}, nil
}

// ListModelRoutes 分页列出 ModelRoute。
func (h *ModelRouteHandler) ListModelRoutes(
	ctx context.Context,
	req *adminv1.ListModelRoutesRequest,
) (*adminv1.ListModelRoutesReply, error) {
	page, err := h.service.List(ctx, modelrouteservice.ListParams{
		Namespace:   req.GetNamespace(),
		PageSize:    req.GetPageSize(),
		PageNum:     req.GetPageNum(),
		DisplayName: req.GetDisplayName(),
		GatewayName: req.GetGatewayName(),
	})
	if err != nil {
		return nil, toModelRouteError(err)
	}

	routes := make([]*adminv1.ModelRoute, len(page.Items))
	for i, route := range page.Items {
		routes[i] = modelRouteToProto(route)
	}

	return &adminv1.ListModelRoutesReply{Routes: routes, TotalCount: page.TotalCount}, nil
}

// UpdateModelRoute 完整替换 ModelRoute 的可编辑配置。
func (h *ModelRouteHandler) UpdateModelRoute(
	ctx context.Context,
	req *adminv1.UpdateModelRouteRequest,
) (*adminv1.UpdateModelRouteReply, error) {
	updated, err := h.service.Update(ctx, modelrouteservice.UpdateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		ParentRefs:  modelRouteParentsFromProto(req.GetParentRefs()),
		Rules:       modelRouteRulesFromProto(req.GetRules()),
	})
	if err != nil {
		return nil, toModelRouteError(err)
	}

	return &adminv1.UpdateModelRouteReply{Route: modelRouteToProto(updated)}, nil
}

// DeleteModelRoute 删除指定 ModelRoute。
func (h *ModelRouteHandler) DeleteModelRoute(
	ctx context.Context,
	req *adminv1.DeleteModelRouteRequest,
) (*adminv1.DeleteModelRouteReply, error) {
	if err := h.service.Delete(ctx, req.GetNamespace(), req.GetName()); err != nil {
		return nil, toModelRouteError(err)
	}

	return &adminv1.DeleteModelRouteReply{}, nil
}

func modelRouteParentsFromProto(refs []*adminv1.ParentReference) []gatewayv1alpha1.ParentReference {
	result := make([]gatewayv1alpha1.ParentReference, len(refs))
	for i, ref := range refs {
		result[i] = gatewayv1alpha1.ParentReference{Name: ref.GetName(), SectionName: ref.GetSectionName()}
	}

	return result
}

func modelRouteRulesFromProto(rules []*adminv1.ModelRouteRule) []gatewayv1alpha1.ModelRouteRule {
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

func modelRouteToProto(route repository.ModelRouteInfo) *adminv1.ModelRoute {
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

func toModelRouteError(err error) error {
	switch {
	case errors.Is(err, modelrouteservice.ErrInvalidArgument):
		return errno.ErrModelRouteInvalidArgument.WithCause(err)
	case errors.Is(err, modelrouteservice.ErrAlreadyExists):
		return errno.ErrModelRouteAlreadyExists.WithCause(err)
	case errors.Is(err, modelrouteservice.ErrNotFound):
		return errno.ErrModelRouteNotFound.WithCause(err)
	case errors.Is(err, modelrouteservice.ErrConflict):
		return errno.ErrModelRouteConflict.WithCause(err)
	default:
		return errno.Fallback(err)
	}
}
