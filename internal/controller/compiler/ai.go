package compiler

import (
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"slices"
	"strconv"
	"strings"

	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/anthropic"
	"github.com/lgc202/ingate/pkg/llm/gemini"
	"github.com/lgc202/ingate/pkg/llm/openai"
	pluginaiproxy "github.com/lgc202/ingate/pkg/plugin/aiproxy"
)

const (
	aiProxyHTTPFilterName     = "ingate.filters.http.ai-proxy"
	aiProxyPluginName         = "ingate.ai-proxy"
	aiProxyPluginPath         = "/opt/ingate/plugins/ai-proxy.wasm"
	openAIChatCompletionsPath = "/v1/chat/completions"
	aiClusterHeader           = "x-ingate-ai-cluster-v1"
	aiRouteHeader             = "x-ingate-ai-route-v1"
	defaultPlainHTTPPort      = 80
	defaultSecureHTTPPort     = 443
	openAIAPIKeyHeader        = "Authorization"
	openAIAPIKeyPrefix        = "Bearer "
	anthropicAPIKeyHeader     = "x-api-key"
	anthropicVersionHeader    = "anthropic-version"
	anthropicVersion          = "2023-06-01"
	geminiAPIKeyHeader        = "x-goog-api-key"
)

var aiManagedRequestHeaders = []string{
	":authority",
	":path",
	"authorization",
	"accept-encoding",
	"content-encoding",
	"content-length",
	"content-type",
	aiClusterHeader,
	aiRouteHeader,
	"x-api-key",
	"x-goog-api-key",
	"anthropic-version",
}

type aiRouteKey struct {
	routeID  string
	ruleName string
}

type compiledAIRoute struct {
	configID     string
	upstreams    []pluginaiproxy.UpstreamConfig
	models       []pluginaiproxy.ModelConfig
	continuation []compiledAIContinuation
}

type compiledAIContinuation struct {
	path      string
	cluster   string
	authority string
}

type compiledAIUpstream struct {
	config    pluginaiproxy.UpstreamConfig
	authority string
}

func (c *compilation) compileAIModels(routeID string, rule gatewayv1.RouteRule, methods []string) (compiledAIRoute, bool) {
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

	items := slices.Clone(rule.ModelRouting.Models)
	slices.SortFunc(items, func(a, b gatewayv1.ModelRoute) int {
		return strings.Compare(a.Model, b.Model)
	})
	upstreamsByID := make(map[string]compiledAIUpstream)
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

		upstreamRef := item.UpstreamRef
		if upstreamRef == "" || strings.TrimSpace(upstreamRef) != upstreamRef {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q model %q has an invalid upstream reference %q", routeID, rule.Name, item.Model, upstreamRef))
			valid = false
			continue
		}
		upstream, exists := c.upstreams[upstreamRef]
		if !exists {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonReferenceNotFound, fmt.Sprintf("route %q rule %q model %q references missing model upstream %q", routeID, rule.Name, item.Model, upstreamRef))
			valid = false
			continue
		}
		if upstream.Spec.Type != gatewayv1.UpstreamTypeModel || upstream.Spec.Model == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidReference, fmt.Sprintf("route %q rule %q model %q references upstream %q without model configuration", routeID, rule.Name, item.Model, upstreamRef))
			valid = false
			continue
		}

		upstreamModel := item.UpstreamModel
		if upstreamModel == "" {
			upstreamModel = item.Model
		}
		if strings.TrimSpace(upstreamModel) != upstreamModel {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q model %q has an invalid upstream model name %q", routeID, rule.Name, item.Model, upstreamModel))
			valid = false
			continue
		}
		if !enabledUpstreamModel(upstream.Spec.Model.Models, upstreamModel) {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidReference, fmt.Sprintf("route %q rule %q model %q references unavailable upstream model %q on %q", routeID, rule.Name, item.Model, upstreamModel, upstreamRef))
			valid = false
			continue
		}

		compiledUpstream, exists := upstreamsByID[upstreamRef]
		if !exists {
			var upstreamValid bool
			compiledUpstream, upstreamValid = c.compileAIUpstream(upstream)
			if !upstreamValid {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindRoute,
					routeID,
					ReasonInvalidReference,
					fmt.Sprintf("route %q rule %q references model upstream %q that could not be compiled", routeID, rule.Name, upstreamRef),
				)
				valid = false
				continue
			}
			upstreamsByID[upstreamRef] = compiledUpstream
		}
		models = append(models, pluginaiproxy.ModelConfig{
			Model:         item.Model,
			UpstreamID:    compiledUpstream.config.ID,
			UpstreamModel: upstreamModel,
		})
	}
	if !valid {
		return compiledAIRoute{}, false
	}

	upstreams := make([]pluginaiproxy.UpstreamConfig, 0, len(upstreamsByID))
	for _, upstreamID := range slices.Sorted(maps.Keys(upstreamsByID)) {
		upstreams = append(upstreams, upstreamsByID[upstreamID].config)
	}
	fields := []string{
		"routeID", routeID,
		"ruleName", rule.Name,
		"clusterHeader", aiClusterHeader,
		"routeHeader", aiRouteHeader,
	}
	for _, upstreamID := range slices.Sorted(maps.Keys(upstreamsByID)) {
		upstream := upstreamsByID[upstreamID]
		fields = append(fields,
			"upstream", upstream.config.ID,
			"protocol", string(upstream.config.Protocol),
			"cluster", upstream.config.Cluster,
			"authority", upstream.authority,
			"basePath", upstream.config.BasePath,
			"apiKeyHeader", upstream.config.APIKeyHeader,
			"apiKeyPrefix", upstream.config.APIKeyPrefix,
			"apiKey", upstream.config.APIKey,
		)
		for _, header := range upstream.config.Headers {
			fields = append(fields, "header", strings.ToLower(header.Name), header.Value)
		}
	}
	for _, model := range models {
		fields = append(fields, "model", model.Model, model.UpstreamID, model.UpstreamModel)
	}
	compiled := compiledAIRoute{
		configID:     configFingerprint(fields...),
		upstreams:    upstreams,
		models:       models,
		continuation: compileAIContinuations(upstreamsByID, models),
	}
	c.aiRoutes[aiRouteKey{routeID: routeID, ruleName: rule.Name}] = compiled
	return compiled, true
}

func (c *compilation) compileAIUpstream(upstream *gatewayv1.Upstream) (compiledAIUpstream, bool) {
	clusterName, clusterExists := c.upstreamClusters[upstream.Name]
	if !clusterExists {
		return compiledAIUpstream{}, false
	}
	apiKey, credentialValid := c.upstreamAPIKey(upstream)
	if !credentialValid {
		return compiledAIUpstream{}, false
	}

	var apiKeyHeader string
	var apiKeyPrefix string
	var headers []pluginaiproxy.HeaderConfig
	// Provider 只用于资源组合校验；数据面执行配置完全由 Protocol 决定
	switch upstream.Spec.Protocol {
	case gatewayv1.UpstreamProtocolOpenAI:
		apiKeyHeader = openAIAPIKeyHeader
		apiKeyPrefix = openAIAPIKeyPrefix
	case gatewayv1.UpstreamProtocolAnthropic:
		apiKeyHeader = anthropicAPIKeyHeader
		headers = []pluginaiproxy.HeaderConfig{{Name: anthropicVersionHeader, Value: anthropicVersion}}
	case gatewayv1.UpstreamProtocolGemini:
		apiKeyHeader = geminiAPIKeyHeader
	default:
		return compiledAIUpstream{}, false
	}
	upstreamConfig := pluginaiproxy.UpstreamConfig{
		ID:       upstream.Name,
		Protocol: llm.Protocol(upstream.Spec.Protocol),
		Cluster:  clusterName,
		BasePath: upstream.Spec.Model.APIBasePath,
		APIKey:   apiKey,
		Headers:  headers,
	}
	if apiKey != "" {
		upstreamConfig.APIKeyHeader = apiKeyHeader
		upstreamConfig.APIKeyPrefix = apiKeyPrefix
	}
	return compiledAIUpstream{
		config:    upstreamConfig,
		authority: modelUpstreamAuthority(upstream),
	}, true
}

func (c *compilation) upstreamAPIKey(upstream *gatewayv1.Upstream) (string, bool) {
	if upstream.Spec.Authentication == nil {
		return "", true
	}
	if upstream.Spec.Authentication.APIKey == nil || upstream.Spec.Authentication.APIKey.Value == "" || !httpheader.ValidValue(upstream.Spec.Authentication.APIKey.Value) {
		return "", false
	}
	return upstream.Spec.Authentication.APIKey.Value, true
}

func (c *compilation) addAIProxyConfigs(configs map[listenerKey]listenerPluginConfig) {
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
			Upstreams:   slices.Clone(aiRoute.upstreams),
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
	return buildWasmHTTPFilter(
		aiProxyHTTPFilterName,
		aiProxyPluginName,
		aiProxyPluginPath,
		raw,
		wasmHTTPFilterOptions{allowOnHeadersStopIteration: true},
	)
}

func enabledUpstreamModel(models []gatewayv1.ModelCatalogItem, name string) bool {
	for _, model := range models {
		if model.Name == name {
			return model.Enabled
		}
	}
	return false
}

func compileAIContinuations(
	upstreams map[string]compiledAIUpstream,
	models []pluginaiproxy.ModelConfig,
) []compiledAIContinuation {
	continuations := make(map[string]compiledAIContinuation)
	for _, model := range models {
		upstream := upstreams[model.UpstreamID]
		for _, requestPath := range modelRequestPaths(upstream.config, model.UpstreamModel) {
			continuation := compiledAIContinuation{
				path:      requestPath,
				cluster:   upstream.config.Cluster,
				authority: upstream.authority,
			}
			continuations[requestPath+"\x00"+upstream.config.Cluster] = continuation
		}
	}

	result := slices.Collect(maps.Values(continuations))
	slices.SortFunc(result, func(a, b compiledAIContinuation) int {
		if a.path != b.path {
			return strings.Compare(a.path, b.path)
		}
		return strings.Compare(a.cluster, b.cluster)
	})
	return result
}

func modelRequestPaths(upstream pluginaiproxy.UpstreamConfig, upstreamModel string) []string {
	switch upstream.Protocol {
	case llm.ProtocolOpenAIChatCompletions:
		return []string{joinModelAPIPath(upstream.BasePath, openai.ChatCompletionsPath)}
	case llm.ProtocolAnthropicMessages:
		return []string{joinModelAPIPath(upstream.BasePath, anthropic.MessagesPath)}
	case llm.ProtocolGeminiGenerateContent:
		requestPath, err := gemini.EndpointPath(upstreamModel, false)
		if err != nil {
			return nil
		}
		streamPath, err := gemini.EndpointPath(upstreamModel, true)
		if err != nil {
			return nil
		}
		streamPath, _, _ = strings.Cut(streamPath, "?")
		return []string{
			joinModelAPIPath(upstream.BasePath, requestPath),
			joinModelAPIPath(upstream.BasePath, streamPath),
		}
	default:
		return nil
	}
}

func joinModelAPIPath(basePath, endpoint string) string {
	if basePath == "/" {
		return endpoint
	}
	return strings.TrimSuffix(basePath, "/") + endpoint
}

func modelUpstreamAuthority(upstream *gatewayv1.Upstream) string {
	endpoints := slices.Clone(upstream.Spec.Endpoints)
	slices.SortFunc(endpoints, func(a, b gatewayv1.Endpoint) int {
		if a.Address != b.Address {
			return strings.Compare(a.Address, b.Address)
		}
		return a.Port - b.Port
	})
	port := 0
	for _, endpoint := range endpoints {
		if endpoint.Enabled {
			port = endpoint.Port
			break
		}
	}
	if upstream.Spec.TLS != nil {
		serverName := normalizedTLSServerName(upstream.Spec.TLS.ServerName)
		if port == 0 || port == defaultSecureHTTPPort {
			return modelAuthorityHost(serverName)
		}
		return net.JoinHostPort(serverName, strconv.Itoa(port))
	}
	for _, endpoint := range endpoints {
		if !endpoint.Enabled {
			continue
		}
		if endpoint.Port == defaultPlainHTTPPort {
			return modelAuthorityHost(endpoint.Address)
		}
		return net.JoinHostPort(endpoint.Address, strconv.Itoa(endpoint.Port))
	}
	return ""
}

func modelAuthorityHost(host string) string {
	if isIPAddress(host) && strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}
