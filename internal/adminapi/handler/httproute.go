package handler

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/errno"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	httprouteservice "github.com/lgc202/ingate/internal/adminapi/service/httproute"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

var (
	_ adminv1.HTTPRouteServiceServer     = (*HTTPRouteHandler)(nil)
	_ adminv1.HTTPRouteServiceHTTPServer = (*HTTPRouteHandler)(nil)
)

// HTTPRouteHandler 实现 HTTPRoute 的 gRPC 和 HTTP 协议入口。
type HTTPRouteHandler struct {
	adminv1.UnimplementedHTTPRouteServiceServer

	service *httprouteservice.Service
}

// NewHTTPRoute 创建 HTTP 路由协议入口，复用注入的业务服务。
func NewHTTPRoute(service *httprouteservice.Service) *HTTPRouteHandler {
	return &HTTPRouteHandler{service: service}
}

// CreateHTTPRoute 创建 HTTPRoute。
func (h *HTTPRouteHandler) CreateHTTPRoute(
	ctx context.Context,
	req *adminv1.CreateHTTPRouteRequest,
) (*adminv1.CreateHTTPRouteReply, error) {
	created, err := h.service.Create(ctx, httprouteservice.CreateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		ParentRefs:  httpRouteParentsFromProto(req.GetParentRefs()),
		Rules:       httpRouteRulesFromProto(req.GetRules()),
	})
	if err != nil {
		return nil, toHTTPRouteError(err)
	}

	return &adminv1.CreateHTTPRouteReply{Route: httpRouteToProto(created)}, nil
}

// GetHTTPRoute 读取指定 HTTPRoute。
func (h *HTTPRouteHandler) GetHTTPRoute(
	ctx context.Context,
	req *adminv1.GetHTTPRouteRequest,
) (*adminv1.GetHTTPRouteReply, error) {
	route, err := h.service.Get(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, toHTTPRouteError(err)
	}

	return &adminv1.GetHTTPRouteReply{Route: httpRouteToProto(route)}, nil
}

// ListHTTPRoutes 分页列出 HTTPRoute。
func (h *HTTPRouteHandler) ListHTTPRoutes(
	ctx context.Context,
	req *adminv1.ListHTTPRoutesRequest,
) (*adminv1.ListHTTPRoutesReply, error) {
	page, err := h.service.List(ctx, httprouteservice.ListParams{
		Namespace:   req.GetNamespace(),
		PageSize:    req.GetPageSize(),
		PageNum:     req.GetPageNum(),
		DisplayName: req.GetDisplayName(),
		GatewayName: req.GetGatewayName(),
	})
	if err != nil {
		return nil, toHTTPRouteError(err)
	}

	routes := make([]*adminv1.HTTPRoute, len(page.Items))
	for i, route := range page.Items {
		routes[i] = httpRouteToProto(route)
	}

	return &adminv1.ListHTTPRoutesReply{Routes: routes, TotalCount: page.TotalCount}, nil
}

// UpdateHTTPRoute 完整替换 HTTPRoute 的可编辑配置。
func (h *HTTPRouteHandler) UpdateHTTPRoute(
	ctx context.Context,
	req *adminv1.UpdateHTTPRouteRequest,
) (*adminv1.UpdateHTTPRouteReply, error) {
	updated, err := h.service.Update(ctx, httprouteservice.UpdateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		ParentRefs:  httpRouteParentsFromProto(req.GetParentRefs()),
		Rules:       httpRouteRulesFromProto(req.GetRules()),
	})
	if err != nil {
		return nil, toHTTPRouteError(err)
	}

	return &adminv1.UpdateHTTPRouteReply{Route: httpRouteToProto(updated)}, nil
}

// DeleteHTTPRoute 删除指定 HTTPRoute。
func (h *HTTPRouteHandler) DeleteHTTPRoute(
	ctx context.Context,
	req *adminv1.DeleteHTTPRouteRequest,
) (*adminv1.DeleteHTTPRouteReply, error) {
	if err := h.service.Delete(ctx, req.GetNamespace(), req.GetName()); err != nil {
		return nil, toHTTPRouteError(err)
	}

	return &adminv1.DeleteHTTPRouteReply{}, nil
}

func httpRouteParentsFromProto(refs []*adminv1.ParentReference) []gatewayv1alpha1.ParentReference {
	result := make([]gatewayv1alpha1.ParentReference, len(refs))
	for i, ref := range refs {
		result[i] = gatewayv1alpha1.ParentReference{Name: ref.GetName(), SectionName: ref.GetSectionName()}
	}

	return result
}

func httpRouteRulesFromProto(rules []*adminv1.HTTPRouteRule) []gatewayv1alpha1.HTTPRouteRule {
	result := make([]gatewayv1alpha1.HTTPRouteRule, len(rules))
	for i, rule := range rules {
		target := &result[i]
		target.Name = rule.GetName()
		for _, match := range rule.GetMatches() {
			var path *gatewayv1alpha1.PathMatch
			if match.GetPath() != nil {
				value := httpRoutePathFromProto(match.GetPath())
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

func httpRoutePathFromProto(path *adminv1.PathMatch) gatewayv1alpha1.PathMatch {
	return gatewayv1alpha1.PathMatch{
		Type:  httpRoutePathTypeFromProto(path.GetType()),
		Value: path.GetValue(),
	}
}

func httpRoutePathTypeFromProto(pathType adminv1.PathMatchType) gatewayv1alpha1.PathMatchType {
	switch pathType {
	case adminv1.PathMatchType_PATH_MATCH_TYPE_EXACT:
		return gatewayv1alpha1.PathMatchExact
	case adminv1.PathMatchType_PATH_MATCH_TYPE_PREFIX:
		return gatewayv1alpha1.PathMatchPrefix
	default:
		return ""
	}
}

func httpRouteToProto(route repository.HTTPRouteInfo) *adminv1.HTTPRoute {
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
				path = &adminv1.PathMatch{Type: httpRoutePathTypeToProto(match.Path.Type), Value: match.Path.Value}
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

func httpRoutePathTypeToProto(pathType gatewayv1alpha1.PathMatchType) adminv1.PathMatchType {
	switch pathType {
	case gatewayv1alpha1.PathMatchExact:
		return adminv1.PathMatchType_PATH_MATCH_TYPE_EXACT
	case gatewayv1alpha1.PathMatchPrefix:
		return adminv1.PathMatchType_PATH_MATCH_TYPE_PREFIX
	default:
		return adminv1.PathMatchType_PATH_MATCH_TYPE_UNSPECIFIED
	}
}

func toHTTPRouteError(err error) error {
	switch {
	case errors.Is(err, httprouteservice.ErrInvalidArgument):
		return errno.ErrHTTPRouteInvalidArgument.WithCause(err)
	case errors.Is(err, httprouteservice.ErrAlreadyExists):
		return errno.ErrHTTPRouteAlreadyExists.WithCause(err)
	case errors.Is(err, httprouteservice.ErrNotFound):
		return errno.ErrHTTPRouteNotFound.WithCause(err)
	case errors.Is(err, httprouteservice.ErrConflict):
		return errno.ErrHTTPRouteConflict.WithCause(err)
	default:
		return errno.Fallback(err)
	}
}
