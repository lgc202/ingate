package convert

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func RouteFromCreateRequest(req dto.CreateRouteRequest) *gatewayv1alpha1.Route {
	return &gatewayv1alpha1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{Name: req.Name},
		Spec: gatewayv1alpha1.RouteSpec{
			ParentRefs: parentRefsFromDTO(req.ParentRefs),
			Hostnames:  append([]string(nil), req.Hostnames...),
			Rules:      routeRulesFromDTO(req.Rules),
		},
	}
}

func RouteFromUpdateRequest(name string, req dto.UpdateRouteRequest) *gatewayv1alpha1.Route {
	return &gatewayv1alpha1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1alpha1.RouteSpec{
			ParentRefs: parentRefsFromDTO(req.ParentRefs),
			Hostnames:  append([]string(nil), req.Hostnames...),
			Rules:      routeRulesFromDTO(req.Rules),
		},
	}
}

func RouteToResponse(route *gatewayv1alpha1.Route) dto.RouteResponse {
	if route == nil {
		return dto.RouteResponse{}
	}
	return dto.RouteResponse{
		Metadata: dto.NewObjectMeta(route.ObjectMeta),
		Spec: dto.RouteSpec{
			ParentRefs: parentRefsToDTO(route.Spec.ParentRefs),
			Hostnames:  append([]string(nil), route.Spec.Hostnames...),
			Rules:      routeRulesToDTO(route.Spec.Rules),
		},
		Status: dto.RouteStatusView{
			ObservedGeneration: route.Status.ObservedGeneration,
			Conditions:         dto.NewConditions(route.Status.Conditions),
			Parents:            routeParentStatusesToDTO(route.Status.Parents),
		},
	}
}

func RouteListToResponse(list *gatewayv1alpha1.RouteList) dto.RouteListResponse {
	if list == nil {
		return dto.RouteListResponse{}
	}
	items := make([]dto.RouteResponse, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, RouteToResponse(&list.Items[i]))
	}
	return dto.RouteListResponse{Items: items}
}

func parentRefsFromDTO(refs []dto.ParentRef) []gatewayv1alpha1.ParentReference {
	items := make([]gatewayv1alpha1.ParentReference, 0, len(refs))
	for _, ref := range refs {
		items = append(items, gatewayv1alpha1.ParentReference{Name: ref.Name})
	}
	return items
}

func parentRefsToDTO(refs []gatewayv1alpha1.ParentReference) []dto.ParentRef {
	items := make([]dto.ParentRef, 0, len(refs))
	for _, ref := range refs {
		items = append(items, dto.ParentRef{Name: ref.Name})
	}
	return items
}

func routeRulesFromDTO(rules []dto.RouteRule) []gatewayv1alpha1.RouteRule {
	items := make([]gatewayv1alpha1.RouteRule, 0, len(rules))
	for _, rule := range rules {
		items = append(items, gatewayv1alpha1.RouteRule{
			Matches:     routeMatchesFromDTO(rule.Matches),
			BackendRefs: backendRefsFromDTO(rule.BackendRefs),
			Filters:     routeFiltersFromDTO(rule.Filters),
		})
	}
	return items
}

func routeRulesToDTO(rules []gatewayv1alpha1.RouteRule) []dto.RouteRule {
	items := make([]dto.RouteRule, 0, len(rules))
	for _, rule := range rules {
		items = append(items, dto.RouteRule{
			Matches:     routeMatchesToDTO(rule.Matches),
			BackendRefs: backendRefsToDTO(rule.BackendRefs),
			Filters:     routeFiltersToDTO(rule.Filters),
		})
	}
	return items
}

func routeMatchesFromDTO(matches []dto.HTTPRouteMatch) []gatewayv1alpha1.HTTPRouteMatch {
	items := make([]gatewayv1alpha1.HTTPRouteMatch, 0, len(matches))
	for _, match := range matches {
		items = append(items, gatewayv1alpha1.HTTPRouteMatch{
			Path:    pathMatchFromDTO(match.Path),
			Method:  match.Method,
			Headers: headerMatchesFromDTO(match.Headers),
		})
	}
	return items
}

func routeMatchesToDTO(matches []gatewayv1alpha1.HTTPRouteMatch) []dto.HTTPRouteMatch {
	items := make([]dto.HTTPRouteMatch, 0, len(matches))
	for _, match := range matches {
		items = append(items, dto.HTTPRouteMatch{
			Path:    pathMatchToDTO(match.Path),
			Method:  match.Method,
			Headers: headerMatchesToDTO(match.Headers),
		})
	}
	return items
}

func pathMatchFromDTO(match *dto.HTTPPathMatch) *gatewayv1alpha1.HTTPPathMatch {
	if match == nil {
		return nil
	}
	return &gatewayv1alpha1.HTTPPathMatch{Type: match.Type, Value: match.Value}
}

func pathMatchToDTO(match *gatewayv1alpha1.HTTPPathMatch) *dto.HTTPPathMatch {
	if match == nil {
		return nil
	}
	return &dto.HTTPPathMatch{Type: match.Type, Value: match.Value}
}

func headerMatchesFromDTO(headers []dto.HTTPHeaderMatch) []gatewayv1alpha1.HTTPHeaderMatch {
	items := make([]gatewayv1alpha1.HTTPHeaderMatch, 0, len(headers))
	for _, header := range headers {
		items = append(items, gatewayv1alpha1.HTTPHeaderMatch{Name: header.Name, Value: header.Value})
	}
	return items
}

func headerMatchesToDTO(headers []gatewayv1alpha1.HTTPHeaderMatch) []dto.HTTPHeaderMatch {
	items := make([]dto.HTTPHeaderMatch, 0, len(headers))
	for _, header := range headers {
		items = append(items, dto.HTTPHeaderMatch{Name: header.Name, Value: header.Value})
	}
	return items
}

func backendRefsFromDTO(refs []dto.BackendRef) []gatewayv1alpha1.BackendRef {
	items := make([]gatewayv1alpha1.BackendRef, 0, len(refs))
	for _, ref := range refs {
		items = append(items, gatewayv1alpha1.BackendRef{Name: ref.Name, Port: ref.Port, Weight: ref.Weight})
	}
	return items
}

func backendRefsToDTO(refs []gatewayv1alpha1.BackendRef) []dto.BackendRef {
	items := make([]dto.BackendRef, 0, len(refs))
	for _, ref := range refs {
		items = append(items, dto.BackendRef{Name: ref.Name, Port: ref.Port, Weight: ref.Weight})
	}
	return items
}

func routeFiltersFromDTO(filters []dto.HTTPRouteFilter) []gatewayv1alpha1.HTTPRouteFilter {
	items := make([]gatewayv1alpha1.HTTPRouteFilter, 0, len(filters))
	for _, filter := range filters {
		items = append(items, gatewayv1alpha1.HTTPRouteFilter{
			Type:                   filter.Type,
			URLRewrite:             urlRewriteFilterFromDTO(filter.URLRewrite),
			RequestHeaderModifier:  headerFilterFromDTO(filter.RequestHeaderModifier),
			ResponseHeaderModifier: headerFilterFromDTO(filter.ResponseHeaderModifier),
		})
	}
	return items
}

func routeFiltersToDTO(filters []gatewayv1alpha1.HTTPRouteFilter) []dto.HTTPRouteFilter {
	items := make([]dto.HTTPRouteFilter, 0, len(filters))
	for _, filter := range filters {
		items = append(items, dto.HTTPRouteFilter{
			Type:                   filter.Type,
			URLRewrite:             urlRewriteFilterToDTO(filter.URLRewrite),
			RequestHeaderModifier:  headerFilterToDTO(filter.RequestHeaderModifier),
			ResponseHeaderModifier: headerFilterToDTO(filter.ResponseHeaderModifier),
		})
	}
	return items
}

func urlRewriteFilterFromDTO(filter *dto.HTTPURLRewriteFilter) *gatewayv1alpha1.HTTPURLRewriteFilter {
	if filter == nil {
		return nil
	}
	return &gatewayv1alpha1.HTTPURLRewriteFilter{Path: pathModifierFromDTO(filter.Path)}
}

func urlRewriteFilterToDTO(filter *gatewayv1alpha1.HTTPURLRewriteFilter) *dto.HTTPURLRewriteFilter {
	if filter == nil {
		return nil
	}
	return &dto.HTTPURLRewriteFilter{Path: pathModifierToDTO(filter.Path)}
}

func pathModifierFromDTO(modifier *dto.HTTPPathModifier) *gatewayv1alpha1.HTTPPathModifier {
	if modifier == nil {
		return nil
	}
	return &gatewayv1alpha1.HTTPPathModifier{
		Type:               modifier.Type,
		ReplacePrefixMatch: modifier.ReplacePrefixMatch,
	}
}

func pathModifierToDTO(modifier *gatewayv1alpha1.HTTPPathModifier) *dto.HTTPPathModifier {
	if modifier == nil {
		return nil
	}
	return &dto.HTTPPathModifier{
		Type:               modifier.Type,
		ReplacePrefixMatch: modifier.ReplacePrefixMatch,
	}
}

func headerFilterFromDTO(filter *dto.HTTPHeaderFilter) *gatewayv1alpha1.HTTPHeaderFilter {
	if filter == nil {
		return nil
	}
	return &gatewayv1alpha1.HTTPHeaderFilter{
		Set:    headersFromDTO(filter.Set),
		Add:    headersFromDTO(filter.Add),
		Remove: append([]string(nil), filter.Remove...),
	}
}

func headerFilterToDTO(filter *gatewayv1alpha1.HTTPHeaderFilter) *dto.HTTPHeaderFilter {
	if filter == nil {
		return nil
	}
	return &dto.HTTPHeaderFilter{
		Set:    headersToDTO(filter.Set),
		Add:    headersToDTO(filter.Add),
		Remove: append([]string(nil), filter.Remove...),
	}
}

func headersFromDTO(headers []dto.HTTPHeader) []gatewayv1alpha1.HTTPHeader {
	items := make([]gatewayv1alpha1.HTTPHeader, 0, len(headers))
	for _, header := range headers {
		items = append(items, gatewayv1alpha1.HTTPHeader{Name: header.Name, Value: header.Value})
	}
	return items
}

func headersToDTO(headers []gatewayv1alpha1.HTTPHeader) []dto.HTTPHeader {
	items := make([]dto.HTTPHeader, 0, len(headers))
	for _, header := range headers {
		items = append(items, dto.HTTPHeader{Name: header.Name, Value: header.Value})
	}
	return items
}

func routeParentStatusesToDTO(parents []gatewayv1alpha1.RouteParentStatus) []dto.RouteParentStatus {
	items := make([]dto.RouteParentStatus, 0, len(parents))
	for _, parent := range parents {
		items = append(items, dto.RouteParentStatus{Name: parent.Name, Conditions: dto.NewConditions(parent.Conditions)})
	}
	return items
}
