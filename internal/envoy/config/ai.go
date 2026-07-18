package config

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/lgc202/ingate/internal/pkg/bearer"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	pluginaiproxy "github.com/lgc202/ingate/pkg/plugin/aiproxy"
)

const (
	aiProxyHTTPFilterName     = "ingate.filters.http.ai-proxy"
	aiProxyPluginName         = "ingate.ai-proxy"
	aiProxyPluginPath         = "/opt/ingate/plugins/ai-proxy.wasm"
	openAIChatCompletionsPath = "/v1/chat/completions"
)

type aiRouteKey struct {
	routeID  string
	ruleName string
}

type compiledAIRoute struct {
	clusterName string
	configID    string
	apiKey      string
	models      []pluginaiproxy.ModelConfig
}

func (c *compileContext) compileAIModels(routeID string, rule gatewayv1.RouteRule, methods []string) (compiledAIRoute, bool) {
	if rule.ModelRouting == nil {
		return compiledAIRoute{}, false
	}
	valid := true
	if len(rule.UpstreamRefs) > 0 {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q cannot declare upstreamRefs and modelRouting together", routeID, rule.Name))
		valid = false
	}
	if len(methods) != 1 || methods[0] != "POST" {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q model routing requires POST as the only method", routeID, rule.Name))
		valid = false
	}
	if rule.PathPrefix != openAIChatCompletionsPath {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q model routing path must be %q", routeID, rule.Name, openAIChatCompletionsPath))
		valid = false
	}
	if rule.Retry != nil {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonUnsupported, fmt.Sprintf("route %q rule %q model routing does not support retry", routeID, rule.Name))
		valid = false
	}
	if len(rule.ModelRouting.Models) == 0 {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q model routing must declare at least one model", routeID, rule.Name))
		valid = false
	}
	upstreamRef := rule.ModelRouting.UpstreamRef
	if upstreamRef == "" || strings.TrimSpace(upstreamRef) != upstreamRef {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q has an invalid model upstream reference %q", routeID, rule.Name, upstreamRef))
		valid = false
	}
	upstream, exists := c.upstreams[upstreamRef]
	if upstreamRef != "" && !exists {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonReferenceNotFound, fmt.Sprintf("route %q rule %q references missing model upstream %q", routeID, rule.Name, upstreamRef))
		valid = false
	}
	if exists && (upstream.Spec.Type != gatewayv1.UpstreamTypeModel || upstream.Spec.Protocol != gatewayv1.UpstreamProtocolOpenAI) {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidReference, fmt.Sprintf("route %q rule %q references upstream %q without OpenAI model protocol", routeID, rule.Name, upstreamRef))
		valid = false
	}
	clusterName, clusterExists := c.upstreamClusters[upstreamRef]
	if exists && !clusterExists {
		valid = false
	}
	apiKey := ""
	if exists {
		var credentialValid bool
		apiKey, credentialValid = c.upstreamAPIKey(upstream)
		if !credentialValid {
			valid = false
		}
	}

	items := slices.Clone(rule.ModelRouting.Models)
	slices.SortFunc(items, func(a, b gatewayv1.ModelRoute) int {
		return strings.Compare(a.Model, b.Model)
	})
	models := make([]pluginaiproxy.ModelConfig, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if item.Model == "" || strings.TrimSpace(item.Model) != item.Model {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q has an invalid client model name %q", routeID, rule.Name, item.Model))
			valid = false
			continue
		}
		if seen[item.Model] {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonConflict, fmt.Sprintf("route %q rule %q declares client model %q more than once", routeID, rule.Name, item.Model))
			valid = false
			continue
		}
		seen[item.Model] = true
		upstreamModel := item.UpstreamModel
		if upstreamModel == "" {
			upstreamModel = item.Model
		}
		if strings.TrimSpace(upstreamModel) != upstreamModel {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q model %q has an invalid upstream model name %q", routeID, rule.Name, item.Model, upstreamModel))
			valid = false
			continue
		}
		models = append(models, pluginaiproxy.ModelConfig{
			Model:         item.Model,
			UpstreamModel: upstreamModel,
		})
	}
	if !valid {
		return compiledAIRoute{}, false
	}
	fields := []string{"clusterName", clusterName, "apiKey", apiKey}
	for _, model := range models {
		fields = append(fields, "model", model.Model, model.UpstreamModel)
	}
	compiled := compiledAIRoute{
		clusterName: clusterName,
		configID:    runtimeConfigID(fields...),
		apiKey:      apiKey,
		models:      models,
	}
	c.aiRoutes[aiRouteKey{routeID: routeID, ruleName: rule.Name}] = compiled
	return compiled, true
}

func (c *compileContext) upstreamAPIKey(upstream *gatewayv1.Upstream) (string, bool) {
	if upstream.Spec.Authentication == nil {
		return "", true
	}
	if upstream.Spec.Authentication.APIKey == nil || !bearer.ValidToken(upstream.Spec.Authentication.APIKey.Value) {
		return "", false
	}
	return upstream.Spec.Authentication.APIKey.Value, true
}

func (c *compileContext) addAIProxyConfigs(configs map[listenerKey]listenerPluginConfig) {
	for _, attachment := range c.routeAttachments {
		aiRoute, exists := c.aiRoutes[aiRouteKey{routeID: attachment.routeID, ruleName: attachment.ruleName}]
		if !exists {
			continue
		}
		config := configs[attachment.listenerKey]
		if config.aiProxy == nil {
			config.aiProxy = &pluginaiproxy.PluginConfig{}
		}
		config.aiProxy.Routes = append(config.aiProxy.Routes, pluginaiproxy.RouteConfig{
			GatewayName: attachment.gatewayID,
			RouteName:   attachment.routeID,
			RuleName:    attachment.ruleName,
			ConfigID:    aiRoute.configID,
			APIKey:      aiRoute.apiKey,
			Models:      slices.Clone(aiRoute.models),
		})
		configs[attachment.listenerKey] = config
	}
}

func buildAIProxyHTTPFilter(config *pluginaiproxy.PluginConfig) (*hcmv3.HttpFilter, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode AI proxy plugin config: %w", err)
	}
	return buildWasmHTTPFilter(aiProxyHTTPFilterName, aiProxyPluginName, aiProxyPluginPath, raw)
}
