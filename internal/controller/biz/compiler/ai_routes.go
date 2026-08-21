package compiler

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const aiRouteNotFoundBody = `{"error":{"message":"The requested model is not published by this route.","type":"invalid_request_error","code":"model_not_found"}}`

// buildAIRouteEntries 把一个 AI Route 展开为模型选择路由和一个初始兜底路由
// 初次选路时请求体中的 model 尚不可见，请求先命中兜底路由；downstream ExtProc 提取模型并清空
// Envoy 路由缓存后，带内部模型 Header 的请求才会命中对应的模型选择路由
func (c *compilation) buildAIRouteEntries(
	route *gatewayv1.Route,
	compiledUpstreams map[string]bool,
) []routeEntry {
	if len(route.Spec.UpstreamRefs) != 0 {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("AI route %q must declare targets in ai.models", route.Name))
		return nil
	}

	path := strings.TrimSpace(route.Spec.Match.Path.Value)
	if path == "" || !strings.HasPrefix(path, "/") {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q path must start with /", route.Name))
		return nil
	}
	exactPath := route.Spec.Match.Path.Type == gatewayv1.PathMatchExact
	if !exactPath && route.Spec.Match.Path.Type != gatewayv1.PathMatchPrefix {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonUnsupported, fmt.Sprintf("route %q uses unsupported path match type %q", route.Name, route.Spec.Match.Path.Type))
		return nil
	}

	methods, methodsValid := c.routeMethods(route)
	headers, headersValid := c.routeHeaderMatches(route)
	requestAdd, requestRemove, requestValid := c.headerModifier(route, route.Spec.RequestHeaderModifier)
	responseAdd, responseRemove, responseValid := c.headerModifier(route, route.Spec.ResponseHeaderModifier)
	if !methodsValid || !headersValid || !requestValid || !responseValid {
		return nil
	}
	if len(methods) != 1 || methods[0] != http.MethodPost {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("AI route %q currently requires POST", route.Name))
		return nil
	}
	for _, header := range route.Spec.Match.Headers {
		if strings.EqualFold(header.Name, aiprotocol.ModelHeader) || strings.EqualFold(header.Name, aiprotocol.UpstreamModelHeader) {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("AI route %q cannot match internal AI headers", route.Name))
			return nil
		}
	}

	perRouteConfig, err := anypb.New(&routev3.FilterConfig{Config: &anypb.Any{}})
	if err != nil {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonCompileFailed, fmt.Sprintf("encode AI filter config for route %q: %v", route.Name, err))
		return nil
	}

	models := slices.Clone(route.Spec.AI.Models)
	slices.SortFunc(models, func(a, b gatewayv1.AIModel) int { return cmp.Compare(a.Name, b.Name) })
	entries := make([]routeEntry, 0, len(models)+1)
	seenModels := make(map[string]bool, len(models))
	for _, model := range models {
		if model.Name == "" || strings.TrimSpace(model.Name) != model.Name || seenModels[model.Name] {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("AI route %q has invalid or duplicate client model %q", route.Name, model.Name))
			continue
		}
		seenModels[model.Name] = true

		clusters, ok := c.aiModelClusters(route, model, compiledUpstreams)
		if !ok {
			continue
		}
		action, ok := c.routeAction(route, clusters)
		if !ok {
			continue
		}
		routeHeaders := append(slices.Clone(headers),
			exactHeaderMatcher(":method", http.MethodPost),
			exactHeaderMatcher(aiprotocol.ModelHeader, model.Name),
		)
		match := &routev3.RouteMatch{Headers: routeHeaders}
		if exactPath {
			match.PathSpecifier = &routev3.RouteMatch_Path{Path: path}
		} else {
			match.PathSpecifier = &routev3.RouteMatch_Prefix{Prefix: path}
		}
		entries = append(entries, routeEntry{
			routeID:     route.Name,
			variant:     "model/" + model.Name,
			path:        path,
			exactPath:   exactPath,
			method:      http.MethodPost,
			headerCount: len(headers) + 1,
			route: &routev3.Route{
				Match:                   match,
				Action:                  &routev3.Route_Route{Route: proto.Clone(action).(*routev3.RouteAction)},
				RequestHeadersToAdd:     requestAdd,
				RequestHeadersToRemove:  requestRemove,
				ResponseHeadersToAdd:    responseAdd,
				ResponseHeadersToRemove: responseRemove,
				TypedPerFilterConfig: map[string]*anypb.Any{
					httpAIDownstreamExtProcFilterName: perRouteConfig,
				},
			},
		})
	}

	if len(models) == 0 {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("AI route %q must publish at least one client model", route.Name))
		return nil
	}

	// 未知模型在重新选路后仍会落到这里，返回稳定的 OpenAI 兼容错误而不是随机选择线路
	fallbackHeaders := append(slices.Clone(headers), exactHeaderMatcher(":method", http.MethodPost))
	fallbackMatch := &routev3.RouteMatch{Headers: fallbackHeaders}
	if exactPath {
		fallbackMatch.PathSpecifier = &routev3.RouteMatch_Path{Path: path}
	} else {
		fallbackMatch.PathSpecifier = &routev3.RouteMatch_Prefix{Prefix: path}
	}
	entries = append(entries, routeEntry{
		routeID:     route.Name,
		variant:     "model-not-found",
		path:        path,
		exactPath:   exactPath,
		method:      http.MethodPost,
		headerCount: len(headers),
		route: &routev3.Route{
			Match: fallbackMatch,
			Action: &routev3.Route_DirectResponse{DirectResponse: &routev3.DirectResponseAction{
				Status: http.StatusNotFound,
				Body: &corev3.DataSource{Specifier: &corev3.DataSource_InlineString{
					InlineString: aiRouteNotFoundBody,
				}},
			}},
			ResponseHeadersToAdd: []*corev3.HeaderValueOption{{
				Header:       &corev3.HeaderValue{Key: "content-type", Value: "application/json"},
				AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			}},
			TypedPerFilterConfig: map[string]*anypb.Any{
				httpAIDownstreamExtProcFilterName: perRouteConfig,
			},
		},
	})
	return entries
}

func (c *compilation) aiModelClusters(
	route *gatewayv1.Route,
	model gatewayv1.AIModel,
	compiledUpstreams map[string]bool,
) ([]*routev3.WeightedCluster_ClusterWeight, bool) {
	if len(model.Targets) == 0 {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("AI route %q client model %q must declare at least one target", route.Name, model.Name))
		return nil, false
	}
	targets := slices.Clone(model.Targets)
	slices.SortFunc(targets, func(a, b gatewayv1.AIModelTarget) int {
		if result := cmp.Compare(a.UpstreamRef, b.UpstreamRef); result != 0 {
			return result
		}
		return cmp.Compare(a.Model, b.Model)
	})

	clusters := make([]*routev3.WeightedCluster_ClusterWeight, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	valid := true
	for _, target := range targets {
		upstream, exists := c.upstreams[target.UpstreamRef]
		compiled := compiledUpstreams[target.UpstreamRef]
		if target.UpstreamRef == "" || seen[target.UpstreamRef] || !exists || !compiled || upstream.Spec.Model == nil ||
			target.Model == "" || strings.TrimSpace(target.Model) != target.Model || target.Weight < 1 || target.Weight > 1000 {
			reason := ReasonInvalidSpec
			if target.UpstreamRef != "" && !exists {
				reason = ReasonReferenceNotFound
			}
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, reason, fmt.Sprintf("AI route %q client model %q has invalid model target %q", route.Name, model.Name, target.UpstreamRef))
			valid = false
			continue
		}
		seen[target.UpstreamRef] = true
		clusters = append(clusters, &routev3.WeightedCluster_ClusterWeight{
			Name:   target.UpstreamRef,
			Weight: wrapperspb.UInt32(uint32(target.Weight)),
			// 真实模型由加权线路写入，因此同一个模型 Service 可以承载多个模型
			RequestHeadersToAdd: []*corev3.HeaderValueOption{{
				Header:       &corev3.HeaderValue{Key: aiprotocol.UpstreamModelHeader, Value: target.Model},
				AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			}},
		})
	}
	return clusters, valid
}
