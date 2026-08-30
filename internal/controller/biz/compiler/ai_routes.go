package compiler

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

const aiModelNotFoundBody = `{"error":{` +
	`"message":"The requested model is not published by this route.",` +
	`"type":"invalid_request_error","code":"model_not_found"}}`

// buildAIRouteEntries 把一个 AI Route 展开为模型选择路由和一个初始兜底路由。
// 初次选路时请求体中的 model 尚不可见，请求先命中兜底路由；
// downstream ExtProc 提取模型并清空。
// Envoy 路由缓存后，带内部模型 Header 的请求才会命中对应的模型选择路由。
func (c *compilation) buildAIRouteEntries(
	route *gatewayv1.Route,
	compiledUpstreams map[string]bool,
) []routeEntry {
	if len(route.Spec.UpstreamRefs) != 0 {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("AI route %q must declare targets in ai.models", route.Name),
		)
		return nil
	}
	if len(route.Spec.AI.Models) == 0 || len(route.Spec.AI.Models) > routeconfig.MaxAIModels {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("AI route %q declares an invalid number of client models", route.Name),
		)
		return nil
	}

	template, ok := c.buildRouteEntryTemplate(route)
	if !ok {
		return nil
	}
	if len(template.methods) != 1 || template.methods[0] != http.MethodPost {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("AI route %q currently requires POST", route.Name),
		)
		return nil
	}
	if routeUsesReservedAIHeader(route) {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("AI route %q uses headers reserved by AI routing", route.Name),
		)
		return nil
	}

	aiFilterConfig, err := anypb.New(&routev3.FilterConfig{Config: &anypb.Any{}})
	if err != nil {
		c.addRouteError(
			route.Name,
			ReasonCompileFailed,
			fmt.Sprintf("encode AI filter config for route %q: %v", route.Name, err),
		)
		return nil
	}

	models := slices.Clone(route.Spec.AI.Models)
	slices.SortFunc(models, func(a, b gatewayv1.AIModel) int { return cmp.Compare(a.Name, b.Name) })
	entries := make([]routeEntry, 0, len(models)+1)
	seenModelNames := make(map[string]bool, len(models))
	valid := true
	for _, model := range models {
		if !routeconfig.IsValidModelName(model.Name) || seenModelNames[model.Name] {
			c.addRouteError(
				route.Name,
				ReasonInvalidSpec,
				fmt.Sprintf(
					"AI route %q has invalid or duplicate client model %q",
					route.Name,
					model.Name,
				),
			)
			valid = false
			continue
		}
		seenModelNames[model.Name] = true

		entry, ok := c.buildAIModelRouteEntry(
			route,
			model,
			template,
			aiFilterConfig,
			compiledUpstreams,
		)
		if !ok {
			valid = false
			continue
		}
		entries = append(entries, entry)
	}
	if !valid {
		return nil
	}

	return append(entries, aiModelNotFoundRouteEntry(route.Name, template, aiFilterConfig))
}

func (c *compilation) buildAIModelRouteEntry(
	route *gatewayv1.Route,
	model gatewayv1.AIModel,
	template routeEntryTemplate,
	aiFilterConfig *anypb.Any,
	compiledUpstreams map[string]bool,
) (routeEntry, bool) {
	clusters, ok := c.buildAIModelClusters(route, model, compiledUpstreams)
	if !ok {
		return routeEntry{}, false
	}
	action, ok := c.buildRouteAction(route, clusters)
	if !ok {
		return routeEntry{}, false
	}
	match := template.match(
		http.MethodPost,
		exactHeaderMatcher(aiprotocol.ModelHeader, model.Name),
	)
	return routeEntry{
		routeID:      route.Name,
		variant:      "model/" + model.Name,
		path:         template.path,
		exactPath:    template.exactPath,
		method:       http.MethodPost,
		headerCount:  len(template.headers) + 1,
		matchHeaders: routeMatchHeaderValues(match),
		route: &routev3.Route{
			Match:                   match,
			Action:                  &routev3.Route_Route{Route: action},
			RequestHeadersToAdd:     template.requestHeadersToAdd,
			RequestHeadersToRemove:  template.requestHeadersToRemove,
			ResponseHeadersToAdd:    template.responseHeadersToAdd,
			ResponseHeadersToRemove: template.responseHeadersToRemove,
			TypedPerFilterConfig: map[string]*anypb.Any{
				httpAIDownstreamExtProcFilterName: aiFilterConfig,
			},
		},
	}, true
}

// aiModelNotFoundRouteEntry 返回未知模型的 OpenAI 兼容错误，
// 防止重新选路失败后随机选择一条模型线路。
func aiModelNotFoundRouteEntry(
	routeID string,
	template routeEntryTemplate,
	aiFilterConfig *anypb.Any,
) routeEntry {
	match := template.match(http.MethodPost)
	return routeEntry{
		routeID:      routeID,
		variant:      "model-not-found",
		path:         template.path,
		exactPath:    template.exactPath,
		method:       http.MethodPost,
		headerCount:  len(template.headers),
		matchHeaders: routeMatchHeaderValues(match),
		route: &routev3.Route{
			Match: match,
			Action: &routev3.Route_DirectResponse{DirectResponse: &routev3.DirectResponseAction{
				Status: http.StatusNotFound,
				Body: &corev3.DataSource{Specifier: &corev3.DataSource_InlineString{
					InlineString: aiModelNotFoundBody,
				}},
			}},
			ResponseHeadersToAdd: []*corev3.HeaderValueOption{{
				Header:       &corev3.HeaderValue{Key: "content-type", Value: "application/json"},
				AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			}},
			TypedPerFilterConfig: map[string]*anypb.Any{
				httpAIDownstreamExtProcFilterName: aiFilterConfig,
			},
		},
	}
}

func (c *compilation) buildAIModelClusters(
	route *gatewayv1.Route,
	model gatewayv1.AIModel,
	compiledUpstreams map[string]bool,
) ([]*routev3.WeightedCluster_ClusterWeight, bool) {
	if len(model.Targets) == 0 {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"AI route %q client model %q must declare at least one target",
				route.Name,
				model.Name,
			),
		)
		return nil, false
	}
	if len(model.Targets) > routeconfig.MaxAIModelTargets {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("AI route %q client model %q has too many targets", route.Name, model.Name),
		)
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
	seenUpstreamIDs := make(map[string]bool, len(targets))
	valid := true
	for _, target := range targets {
		upstream, exists := c.upstreams[target.UpstreamRef]
		compiledSuccessfully := compiledUpstreams[target.UpstreamRef]
		duplicateUpstream := seenUpstreamIDs[target.UpstreamRef]
		if target.UpstreamRef != "" {
			seenUpstreamIDs[target.UpstreamRef] = true
		}

		var reason Reason
		var message string
		switch {
		case !resourceconfig.IsCanonicalID(target.UpstreamRef) || duplicateUpstream ||
			!routeconfig.IsValidModelName(target.Model) ||
			target.Weight < routeconfig.MinTargetWeight ||
			target.Weight > routeconfig.MaxTargetWeight:
			reason = ReasonInvalidSpec
			message = fmt.Sprintf(
				"AI route %q client model %q has invalid model target %q",
				route.Name,
				model.Name,
				target.UpstreamRef,
			)
		case !exists:
			reason = ReasonReferenceNotFound
			message = fmt.Sprintf(
				"AI route %q client model %q references missing upstream %q",
				route.Name,
				model.Name,
				target.UpstreamRef,
			)
		case !compiledSuccessfully:
			reason = ReasonInvalidReference
			message = fmt.Sprintf(
				"AI route %q client model %q references invalid upstream %q",
				route.Name,
				model.Name,
				target.UpstreamRef,
			)
		case upstream.Spec.Model == nil:
			reason = ReasonInvalidReference
			message = fmt.Sprintf(
				"AI route %q client model %q references non-model upstream %q",
				route.Name,
				model.Name,
				target.UpstreamRef,
			)
		default:
			clusters = append(clusters, &routev3.WeightedCluster_ClusterWeight{
				Name:   target.UpstreamRef,
				Weight: wrapperspb.UInt32(uint32(target.Weight)),
				// 真实模型由加权线路写入，因此同一个模型 Service 可以承载多个模型。
				RequestHeadersToAdd: []*corev3.HeaderValueOption{{
					Header: &corev3.HeaderValue{
						Key:   aiprotocol.UpstreamModelHeader,
						Value: target.Model,
					},
					AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
				}},
			})
			continue
		}

		c.addRouteError(
			route.Name,
			reason,
			message,
		)
		valid = false
	}
	return clusters, valid
}

func routeUsesReservedAIHeader(route *gatewayv1.Route) bool {
	for _, header := range route.Spec.Match.Headers {
		if aiprotocol.IsInternalHeader(header.Name) {
			return true
		}
	}
	modifier := route.Spec.RequestHeaderModifier
	if modifier == nil {
		return false
	}
	for _, header := range modifier.Set {
		if aiprotocol.IsInternalHeader(header.Name) {
			return true
		}
	}
	for _, header := range modifier.Add {
		if aiprotocol.IsInternalHeader(header.Name) {
			return true
		}
	}
	for _, name := range modifier.Remove {
		if aiprotocol.IsInternalHeader(name) {
			return true
		}
	}
	return false
}
