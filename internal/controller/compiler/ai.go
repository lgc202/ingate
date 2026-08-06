package compiler

import (
	"fmt"
	"maps"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	mutationrulesv3 "github.com/envoyproxy/go-control-plane/envoy/config/common/mutation_rules/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprochttpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/lgc202/ingate/internal/pkg/aiproxyconfig"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/pkg/llm"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	aiProxyHTTPFilterName     = "envoy.filters.http.ext_proc"
	aiProxyClusterName        = "ingate-system-ai-proxy"
	openAIChatCompletionsPath = "/v1/chat/completions"
	aiClusterHeader           = "x-ingate-ai-cluster-v1"
	defaultPlainHTTPPort      = 80
	defaultSecureHTTPPort     = 443
	openAIAPIKeyHeader        = "Authorization"
	openAIAPIKeyPrefix        = "Bearer "
	anthropicAPIKeyHeader     = "x-api-key"
	anthropicVersionHeader    = "anthropic-version"
	anthropicVersion          = "2023-06-01"
	geminiAPIKeyHeader        = "x-goog-api-key"
	aiProxyMessageTimeout     = 2 * time.Second
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
	"x-api-key",
	"x-goog-api-key",
	"anthropic-version",
}

type aiRouteKey struct {
	routeID  string
	ruleName string
}

type attachedAIRouteKey struct {
	listenerKey listenerKey
	gatewayID   string
	routeID     string
	ruleName    string
}

type compiledAIRoute struct {
	upstreams []aiproxyconfig.Upstream
	models    []aiproxyconfig.Model
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
	upstreamsByID := make(map[string]aiproxyconfig.Upstream)
	models := make([]aiproxyconfig.Model, 0, len(items))
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
		models = append(models, aiproxyconfig.Model{
			Model:         item.Model,
			UpstreamID:    compiledUpstream.ID,
			UpstreamModel: upstreamModel,
		})
	}
	if !valid {
		return compiledAIRoute{}, false
	}

	upstreams := make([]aiproxyconfig.Upstream, 0, len(upstreamsByID))
	for _, upstreamID := range slices.Sorted(maps.Keys(upstreamsByID)) {
		upstreams = append(upstreams, upstreamsByID[upstreamID])
	}
	compiled := compiledAIRoute{upstreams: upstreams, models: models}
	c.aiRoutes[aiRouteKey{routeID: routeID, ruleName: rule.Name}] = compiled
	return compiled, true
}

func (c *compilation) compileAIUpstream(upstream *gatewayv1.Upstream) (aiproxyconfig.Upstream, bool) {
	clusterName, clusterExists := c.upstreamClusters[upstream.Name]
	if !clusterExists {
		return aiproxyconfig.Upstream{}, false
	}
	apiKey, credentialValid := c.upstreamAPIKey(upstream)
	if !credentialValid {
		return aiproxyconfig.Upstream{}, false
	}

	var apiKeyHeader string
	var apiKeyPrefix string
	var headers []aiproxyconfig.Header
	// Provider 只用于资源组合校验；数据面执行配置完全由 Protocol 决定
	switch upstream.Spec.Protocol {
	case gatewayv1.UpstreamProtocolOpenAI:
		apiKeyHeader = openAIAPIKeyHeader
		apiKeyPrefix = openAIAPIKeyPrefix
	case gatewayv1.UpstreamProtocolAnthropic:
		apiKeyHeader = anthropicAPIKeyHeader
		headers = []aiproxyconfig.Header{{Name: anthropicVersionHeader, Value: anthropicVersion}}
	case gatewayv1.UpstreamProtocolGemini:
		apiKeyHeader = geminiAPIKeyHeader
	default:
		return aiproxyconfig.Upstream{}, false
	}
	config := aiproxyconfig.Upstream{
		ID:        upstream.Name,
		Protocol:  llm.Protocol(upstream.Spec.Protocol),
		Cluster:   clusterName,
		Authority: modelUpstreamAuthority(upstream),
		BasePath:  upstream.Spec.Model.APIBasePath,
		APIKey:    apiKey,
		Headers:   headers,
	}
	if apiKey != "" {
		config.APIKeyHeader = apiKeyHeader
		config.APIKeyPrefix = apiKeyPrefix
	}
	return config, true
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

func (c *compilation) configureAIRoutes(configs map[listenerKey]listenerFilterConfig) {
	for _, attachment := range c.routeAttachments {
		aiRoute, exists := c.aiRoutes[aiRouteKey{routeID: attachment.routeID, ruleName: attachment.ruleName}]
		if !exists {
			continue
		}
		listenerConfig := configs[attachment.listenerKey]
		encodedConfig, err := aiproxyconfig.Encode(aiproxyconfig.Config{
			RequireUsage: listenerConfig.requiresAIUsage(
				attachment.gatewayID,
				attachment.routeID,
				attachment.ruleName,
			),
			Upstreams: slices.Clone(aiRoute.upstreams),
			Models:    slices.Clone(aiRoute.models),
		})
		if err != nil {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				attachment.routeID,
				ReasonCompileFailed,
				fmt.Sprintf("route %q rule %q AI execution config is invalid: %v", attachment.routeID, attachment.ruleName, err),
			)
			continue
		}
		key := attachedAIRouteKey{
			listenerKey: attachment.listenerKey,
			gatewayID:   attachment.gatewayID,
			routeID:     attachment.routeID,
			ruleName:    attachment.ruleName,
		}
		typedRouteConfig, err := buildAIProxyRouteConfig(encodedConfig)
		if err != nil {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				attachment.routeID,
				ReasonCompileFailed,
				fmt.Sprintf("route %q rule %q AI proxy config could not be encoded: %v", attachment.routeID, attachment.ruleName, err),
			)
			continue
		}
		for _, route := range c.aiRouteEntries[key] {
			if route.TypedPerFilterConfig == nil {
				route.TypedPerFilterConfig = make(map[string]*anypb.Any)
			}
			route.TypedPerFilterConfig[aiProxyHTTPFilterName] = typedRouteConfig
		}
		listenerConfig.aiProxy = true
		configs[attachment.listenerKey] = listenerConfig
	}
}

func buildAIProxyHTTPFilter() (*hcmv3.HttpFilter, error) {
	config := &extprochttpv3.ExternalProcessor{
		GrpcService: &corev3.GrpcService{
			TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{
					ClusterName:             aiProxyClusterName,
					MaxReceiveMessageLength: wrapperspb.UInt32(aiproxyconfig.ResponseBufferLimitBytes),
				},
			},
		},
		ProcessingMode: aiProxyProcessingMode(),
		MessageTimeout: durationpb.New(aiProxyMessageTimeout),
		StatPrefix:     "ingate_ai_proxy",
		StatusOnError:  &typev3.HttpStatus{Code: typev3.StatusCode_ServiceUnavailable},
		MutationRules: &mutationrulesv3.HeaderMutationRules{
			AllowAllRouting: wrapperspb.Bool(true),
			DisallowIsError: wrapperspb.Bool(true),
		},
		AllowModeOverride: true,
	}
	if err := config.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate Envoy AI proxy filter: %w", err)
	}
	typedConfig, err := anypb.New(config)
	if err != nil {
		return nil, fmt.Errorf("encode Envoy AI proxy filter: %w", err)
	}
	return &hcmv3.HttpFilter{
		Name:     aiProxyHTTPFilterName,
		Disabled: true,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: typedConfig,
		},
	}, nil
}

func buildAIProxyRouteConfig(encodedConfig string) (*anypb.Any, error) {
	config := &extprochttpv3.ExtProcPerRoute{
		Override: &extprochttpv3.ExtProcPerRoute_Overrides{
			Overrides: &extprochttpv3.ExtProcOverrides{
				ProcessingMode: aiProxyProcessingMode(),
				GrpcInitialMetadata: []*corev3.HeaderValue{{
					Key:   aiproxyconfig.GRPCMetadataKey,
					Value: encodedConfig,
				}},
			},
		},
	}
	if err := config.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate Envoy AI proxy route config: %w", err)
	}
	typedConfig, err := anypb.New(config)
	if err != nil {
		return nil, fmt.Errorf("encode Envoy AI proxy route config: %w", err)
	}
	return typedConfig, nil
}

func aiProxyProcessingMode() *extprochttpv3.ProcessingMode {
	return &extprochttpv3.ProcessingMode{
		RequestHeaderMode:   extprochttpv3.ProcessingMode_SEND,
		ResponseHeaderMode:  extprochttpv3.ProcessingMode_SEND,
		RequestBodyMode:     extprochttpv3.ProcessingMode_BUFFERED,
		ResponseBodyMode:    extprochttpv3.ProcessingMode_NONE,
		RequestTrailerMode:  extprochttpv3.ProcessingMode_SKIP,
		ResponseTrailerMode: extprochttpv3.ProcessingMode_SKIP,
	}
}

func enabledUpstreamModel(models []gatewayv1.ModelCatalogItem, name string) bool {
	for _, model := range models {
		if model.Name == name {
			return model.Enabled
		}
	}
	return false
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
