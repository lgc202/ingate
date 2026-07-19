package compiler

import (
	"slices"
	"strings"
	"testing"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	httpwasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/wrapperspb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/pkg/llm"
	pluginaiproxy "github.com/lgc202/ingate/pkg/plugin/aiproxy"
)

func TestCompilerBuildsOpenAIModelRoute(t *testing.T) {
	result := Compile(newAICompilerResources())
	if result.HasErrors() {
		t.Fatalf("Compile(OpenAI model route) diagnostics = %v, want no errors", result.Diagnostics)
	}

	config := decodeAIProxyConfig(t, findCompiledListener(t, result.Config.Listeners, "ingate/http-8080"))
	if got, want := len(config.Routes), 1; got != want {
		t.Fatalf("Compile(OpenAI model route) AI proxy route count = %d, want %d", got, want)
	}
	pluginRoute := config.Routes[0]
	if pluginRoute.ConfigID == "" {
		t.Fatal("Compile(OpenAI model route) AI proxy config ID is empty")
	}

	routeName := envoyAIRouteName("ai-gateway", "ai-route", "chat", "POST", pluginRoute.ConfigID)
	route := findCompiledRoute(t, result.Config.Routes, routeName)
	if got, want := route.GetMatch().GetPath(), openAIChatCompletionsPath; got != want {
		t.Errorf("Compile(OpenAI model route) exact path = %q, want %q", got, want)
	}
	action := route.GetRoute()
	if got, want := action.GetClusterHeader(), aiClusterHeader; got != want {
		t.Errorf("Compile(OpenAI model route) cluster header = %q, want %q", got, want)
	}
	if !slices.Contains(route.RequestHeadersToRemove, aiClusterHeader) {
		t.Errorf("Compile(OpenAI model route) request headers to remove = %v, want %q", route.RequestHeadersToRemove, aiClusterHeader)
	}
	if got, want := len(pluginRoute.Upstreams), 1; got != want {
		t.Fatalf("Compile(OpenAI model route) AI proxy upstream count = %d, want %d", got, want)
	}
	upstream := pluginRoute.Upstreams[0]
	if !strings.HasPrefix(upstream.Cluster, "model-upstream/ai/") {
		t.Errorf("Compile(OpenAI model route) upstream cluster = %q, want prefix %q", upstream.Cluster, "model-upstream/ai/")
	}
	cluster := findCompiledCluster(t, result.Config.Clusters, upstream.Cluster)
	if got, want := cluster.GetEdsClusterConfig().GetServiceName(), cluster.Name; got != want {
		t.Errorf("Compile(OpenAI model route) EDS service name = %q, want %q", got, want)
	}
	assignment := findCompiledEndpoint(t, result.Config.Endpoints, cluster.Name)
	if got, want := assignment.ClusterName, cluster.Name; got != want {
		t.Errorf("Compile(OpenAI model route) EDS cluster name = %q, want %q", got, want)
	}
	if action.GetTimeout() == nil {
		t.Fatal("Compile(OpenAI model route) timeout = nil, want explicit zero timeout")
	}
	if got, want := action.GetTimeout().AsDuration(), time.Duration(0); got != want {
		t.Errorf("Compile(OpenAI model route) timeout = %v, want %v", got, want)
	}

	if got, want := pluginRoute.GatewayName, "ai-gateway"; got != want {
		t.Errorf("Compile(OpenAI model route) AI proxy gateway = %q, want %q", got, want)
	}
	if got, want := pluginRoute.RouteName, "ai-route"; got != want {
		t.Errorf("Compile(OpenAI model route) AI proxy route = %q, want %q", got, want)
	}
	if got, want := pluginRoute.RuleName, "chat"; got != want {
		t.Errorf("Compile(OpenAI model route) AI proxy rule = %q, want %q", got, want)
	}
	if got, want := upstream.APIKey, "sk-test-secret"; got != want {
		t.Errorf("Compile(OpenAI model route) API key = %q, want %q", got, want)
	}
	if got, want := upstream.APIKeyHeader, "Authorization"; got != want {
		t.Errorf("Compile(OpenAI model route) API key header = %q, want %q", got, want)
	}
	if got, want := len(pluginRoute.Models), 1; got != want {
		t.Fatalf("Compile(OpenAI model route) AI proxy model count = %d, want %d", got, want)
	}
	model := pluginRoute.Models[0]
	if got, want := model.Model, "assistant"; got != want {
		t.Errorf("Compile(OpenAI model route) client model = %q, want %q", got, want)
	}
	if got, want := model.UpstreamModel, "assistant"; got != want {
		t.Errorf("Compile(OpenAI model route) default upstream model = %q, want %q", got, want)
	}
	if got, want := model.UpstreamID, upstream.ID; got != want {
		t.Errorf("Compile(OpenAI model route) model upstream = %q, want %q", got, want)
	}

	routes := findCompiledRoutes(t, result.Config.Routes, routeName)
	if got, want := len(routes), 2; got != want {
		t.Fatalf("Compile(OpenAI model route) xDS route count = %d, want public and continuation routes", got)
	}
	continuation := findAIContinuationRoute(t, routes, pluginRoute.ConfigID, upstream.Cluster)
	if got, want := continuation.GetRoute().GetHostRewriteLiteral(), "api.example.com:8080"; got != want {
		t.Errorf("Compile(OpenAI model route) continuation host rewrite = %q, want %q", got, want)
	}
	routeConfig := findRouteConfiguration(t, result.Config.Routes, routeName)
	for _, header := range []string{aiClusterHeader, aiRouteHeader} {
		if !slices.Contains(routeConfig.InternalOnlyHeaders, header) {
			t.Errorf("Compile(OpenAI model route) internal headers = %v, want %q", routeConfig.InternalOnlyHeaders, header)
		}
		if !slices.Contains(continuation.RequestHeadersToRemove, header) {
			t.Errorf("Compile(OpenAI model route) continuation headers to remove = %v, want %q", continuation.RequestHeadersToRemove, header)
		}
	}
}

func TestCompilerUsesProtocolForOpenAICompatibleProviders(t *testing.T) {
	providers := []gatewayv1.ModelProvider{
		gatewayv1.ModelProviderOpenAI,
		gatewayv1.ModelProviderDeepSeek,
		gatewayv1.ModelProviderQwen,
		gatewayv1.ModelProviderCustom,
	}
	var wantUpstream pluginaiproxy.UpstreamConfig
	var wantConfigID string
	for i, provider := range providers {
		t.Run(string(provider), func(t *testing.T) {
			resources := newAICompilerResources()
			resources.Upstreams[0].Spec.Model.Provider = provider
			result := Compile(resources)
			if result.HasErrors() {
				t.Fatalf("Compile(OpenAI-compatible provider %q) diagnostics = %v, want no errors", provider, result.Diagnostics)
			}

			config := decodeAIProxyConfig(t, findCompiledListener(t, result.Config.Listeners, "ingate/http-8080"))
			gotUpstream := config.Routes[0].Upstreams[0]
			if i == 0 {
				wantUpstream = gotUpstream
				wantConfigID = config.Routes[0].ConfigID
				return
			}
			if diff := cmp.Diff(wantUpstream, gotUpstream); diff != "" {
				t.Errorf("Compile(OpenAI-compatible provider %q) upstream mismatch (-want +got):\n%s", provider, diff)
			}
			if got := config.Routes[0].ConfigID; got != wantConfigID {
				t.Errorf("Compile(OpenAI-compatible provider %q) config ID = %q, want %q", provider, got, wantConfigID)
			}
		})
	}
}

func TestCompilerBuildsCrossProtocolModelRoute(t *testing.T) {
	resources := newAICompilerResources()
	anthropicUpstream := newAIUpstream("anthropic-upstream", "anthropic-secret")
	anthropicUpstream.Spec.Protocol = gatewayv1.UpstreamProtocolAnthropic
	anthropicUpstream.Spec.TLS.ServerName = "api.anthropic.com"
	anthropicUpstream.Spec.Endpoints[0].Address = "192.0.2.11"
	anthropicUpstream.Spec.Endpoints[0].Port = 443
	anthropicUpstream.Spec.Model = &gatewayv1.ModelSpec{
		Provider:    gatewayv1.ModelProviderAnthropic,
		APIBasePath: "/v1",
		Models: []gatewayv1.ModelCatalogItem{
			{Name: "claude-sonnet", DisplayName: "Claude Sonnet", Enabled: true},
		},
	}
	geminiUpstream := newAIUpstream("gemini-upstream", "gemini-secret")
	geminiUpstream.Spec.Protocol = gatewayv1.UpstreamProtocolGemini
	geminiUpstream.Spec.TLS.ServerName = "generativelanguage.googleapis.com"
	geminiUpstream.Spec.Endpoints[0].Address = "192.0.2.12"
	geminiUpstream.Spec.Endpoints[0].Port = 443
	geminiUpstream.Spec.Model = &gatewayv1.ModelSpec{
		Provider:    gatewayv1.ModelProviderGemini,
		APIBasePath: "/v1beta",
		Models: []gatewayv1.ModelCatalogItem{
			{Name: "gemini-2.5-flash", DisplayName: "Gemini 2.5 Flash", Enabled: true},
		},
	}
	resources.Upstreams = append(resources.Upstreams, anthropicUpstream, geminiUpstream)
	resources.Routes[0].Spec.Rules[0].ModelRouting.Models = []gatewayv1.ModelRoute{
		{Model: "assistant", UpstreamRef: "model-upstream", UpstreamModel: "gpt-assistant"},
		{Model: "claude", UpstreamRef: anthropicUpstream.Name, UpstreamModel: "claude-sonnet"},
		{Model: "gemini", UpstreamRef: geminiUpstream.Name, UpstreamModel: "gemini-2.5-flash"},
	}

	result := Compile(resources)
	if result.HasErrors() {
		t.Fatalf("Compile(cross-protocol model route) diagnostics = %v, want no errors", result.Diagnostics)
	}
	config := decodeAIProxyConfig(t, findCompiledListener(t, result.Config.Listeners, "ingate/http-8080"))
	if got, want := len(config.Routes), 1; got != want {
		t.Fatalf("Compile(cross-protocol model route) plugin route count = %d, want %d", got, want)
	}
	pluginRoute := config.Routes[0]
	if got, want := len(pluginRoute.Upstreams), 3; got != want {
		t.Fatalf("Compile(cross-protocol model route) upstream count = %d, want %d", got, want)
	}
	if got, want := len(pluginRoute.Models), 3; got != want {
		t.Fatalf("Compile(cross-protocol model route) model count = %d, want %d", got, want)
	}

	upstreams := make(map[string]pluginaiproxy.UpstreamConfig, len(pluginRoute.Upstreams))
	for _, upstream := range pluginRoute.Upstreams {
		upstreams[upstream.ID] = upstream
		findCompiledCluster(t, result.Config.Clusters, upstream.Cluster)
	}
	assertAIUpstream(t, upstreams["model-upstream"], llm.ProtocolOpenAIChatCompletions, "Authorization", "Bearer ", nil)
	assertAIUpstream(t, upstreams[anthropicUpstream.Name], llm.ProtocolAnthropicMessages, "x-api-key", "", map[string]string{
		"anthropic-version": "2023-06-01",
	})
	assertAIUpstream(t, upstreams[geminiUpstream.Name], llm.ProtocolGeminiGenerateContent, "x-goog-api-key", "", nil)

	routeName := envoyAIRouteName("ai-gateway", "ai-route", "chat", "POST", pluginRoute.ConfigID)
	routes := findCompiledRoutes(t, result.Config.Routes, routeName)
	if got, want := len(routes), 5; got != want {
		t.Fatalf("Compile(cross-protocol model route) xDS route count = %d, want one public and four continuation routes", got)
	}
	wantAuthorities := map[string]string{
		"model-upstream":       "api.example.com:8080",
		anthropicUpstream.Name: "api.anthropic.com",
		geminiUpstream.Name:    "generativelanguage.googleapis.com",
	}
	for _, upstream := range pluginRoute.Upstreams {
		continuation := findAIContinuationRoute(t, routes, pluginRoute.ConfigID, upstream.Cluster)
		if got, want := continuation.GetRoute().GetHostRewriteLiteral(), wantAuthorities[upstream.ID]; got != want {
			t.Errorf("Compile(cross-protocol model route) upstream %q host rewrite = %q, want %q", upstream.ID, got, want)
		}
	}
}

func TestCompilerKeepsAnthropicVersionWithoutAPIKey(t *testing.T) {
	resources := newAICompilerResources()
	upstream := resources.Upstreams[0]
	upstream.Spec.Protocol = gatewayv1.UpstreamProtocolAnthropic
	upstream.Spec.Authentication = nil
	upstream.Spec.Model.Provider = gatewayv1.ModelProviderAnthropic
	result := Compile(resources)
	if result.HasErrors() {
		t.Fatalf("Compile(Anthropic upstream without API key) diagnostics = %v, want no errors", result.Diagnostics)
	}

	config := decodeAIProxyConfig(t, findCompiledListener(t, result.Config.Listeners, "ingate/http-8080"))
	assertAIUpstream(t, config.Routes[0].Upstreams[0], llm.ProtocolAnthropicMessages, "", "", map[string]string{
		anthropicVersionHeader: anthropicVersion,
	})
}

func TestCompilerFormatsIPv6ModelAuthority(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*gatewayv1.Upstream)
	}{
		{
			name: "TLS default port",
			mutate: func(upstream *gatewayv1.Upstream) {
				upstream.Spec.TLS.ServerName = "2001:db8::1"
				upstream.Spec.Endpoints[0].Address = "2001:db8::2"
				upstream.Spec.Endpoints[0].Port = 443
			},
		},
		{
			name: "HTTP default port",
			mutate: func(upstream *gatewayv1.Upstream) {
				upstream.Spec.Authentication = nil
				upstream.Spec.TLS = nil
				upstream.Spec.Endpoints[0].Address = "2001:db8::1"
				upstream.Spec.Endpoints[0].Port = 80
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := newAICompilerResources()
			tt.mutate(resources.Upstreams[0])
			result := Compile(resources)
			if result.HasErrors() {
				t.Fatalf("Compile(%s IPv6 model authority) diagnostics = %v, want no errors", tt.name, result.Diagnostics)
			}

			config := decodeAIProxyConfig(t, findCompiledListener(t, result.Config.Listeners, "ingate/http-8080"))
			pluginRoute := config.Routes[0]
			routeName := envoyAIRouteName("ai-gateway", "ai-route", "chat", "POST", pluginRoute.ConfigID)
			continuation := findAIContinuationRoute(t, findCompiledRoutes(t, result.Config.Routes, routeName), pluginRoute.ConfigID, pluginRoute.Upstreams[0].Cluster)
			if got, want := continuation.GetRoute().GetHostRewriteLiteral(), "[2001:db8::1]"; got != want {
				t.Errorf("Compile(%s IPv6 model authority) host rewrite = %q, want %q", tt.name, got, want)
			}
		})
	}
}

func TestCompareRouteEntriesPrefersExactModelPath(t *testing.T) {
	entries := []routeEntry{
		{routeID: "fallback", pathPrefix: openAIChatCompletionsPath},
		{routeID: "model", pathPrefix: openAIChatCompletionsPath, exactPath: true},
	}
	slices.SortFunc(entries, compareRouteEntries)
	if got, want := entries[0].routeID, "model"; got != want {
		t.Errorf("compareRouteEntries() first route = %q, want %q", got, want)
	}
}

func TestCompilerInjectsAIProxyIntoEveryListener(t *testing.T) {
	result := Compile(newAICompilerResources())
	if result.HasErrors() {
		t.Fatalf("Compile(mixed AI and HTTP listeners) diagnostics = %v, want no errors", result.Diagnostics)
	}

	tests := []struct {
		listenerName    string
		wantFilters     []string
		wantRoutes      int
		wantBufferLimit uint32
	}{
		{
			listenerName:    "ingate/http-8080",
			wantFilters:     []string{aiProxyHTTPFilterName, httpRouterFilterName},
			wantRoutes:      1,
			wantBufferLimit: pluginaiproxy.ResponseBufferLimitBytes,
		},
		{
			listenerName: "ingate/http-8081",
			wantFilters:  []string{aiProxyHTTPFilterName, httpRouterFilterName},
		},
	}
	for _, tt := range tests {
		t.Run(tt.listenerName, func(t *testing.T) {
			listener := findCompiledListener(t, result.Config.Listeners, tt.listenerName)
			if got := listener.GetPerConnectionBufferLimitBytes().GetValue(); got != tt.wantBufferLimit {
				t.Errorf("Compile(mixed AI and HTTP listeners) buffer limit for %q = %d, want %d", tt.listenerName, got, tt.wantBufferLimit)
			}
			manager := decodeHTTPConnectionManager(t, listener)
			gotFilters := make([]string, 0, len(manager.HttpFilters))
			for _, filter := range manager.HttpFilters {
				gotFilters = append(gotFilters, filter.Name)
			}
			if !slices.Equal(gotFilters, tt.wantFilters) {
				t.Errorf("Compile(mixed AI and HTTP listeners) filters for %q = %v, want %v", tt.listenerName, gotFilters, tt.wantFilters)
			}
			config := decodeAIProxyConfig(t, listener)
			if got := len(config.Routes); got != tt.wantRoutes {
				t.Errorf("Compile(mixed AI and HTTP listeners) AI route count for %q = %d, want %d", tt.listenerName, got, tt.wantRoutes)
			}
		})
	}
}

func TestCompilerVersionsAIProxyConfig(t *testing.T) {
	newResources := func() Resources {
		resources := newAICompilerResources()
		resources.Routes[0].Spec.Rules[0].ModelRouting.Models = []gatewayv1.ModelRoute{
			{Model: "assistant", UpstreamRef: "model-upstream", UpstreamModel: "gpt-assistant"},
			{Model: "reasoning", UpstreamRef: "model-upstream", UpstreamModel: "gpt-reasoning"},
		}
		return resources
	}

	baselineCluster, baselineConfigID := compileAITestIdentity(t, newResources())
	tests := []struct {
		name              string
		mutate            func(Resources)
		wantClusterChange bool
		wantConfigChange  bool
	}{
		{
			name: "API key change",
			mutate: func(resources Resources) {
				resources.Upstreams[0].Spec.Authentication.APIKey.Value = "sk-rotated-secret"
			},
			wantClusterChange: true,
			wantConfigChange:  true,
		},
		{
			name: "enabled endpoint change",
			mutate: func(resources Resources) {
				resources.Upstreams[0].Spec.Endpoints[0].Address = "192.0.2.11"
			},
			wantClusterChange: true,
			wantConfigChange:  true,
		},
		{
			name: "model mapping change",
			mutate: func(resources Resources) {
				resources.Routes[0].Spec.Rules[0].ModelRouting.Models[0].UpstreamModel = "gpt-assistant-v2"
			},
			wantConfigChange: true,
		},
		{
			name: "model order change",
			mutate: func(resources Resources) {
				models := resources.Routes[0].Spec.Rules[0].ModelRouting.Models
				models[0], models[1] = models[1], models[0]
			},
		},
		{
			name: "provider preset change",
			mutate: func(resources Resources) {
				resources.Upstreams[0].Spec.Model.Provider = gatewayv1.ModelProviderDeepSeek
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := newResources()
			tt.mutate(resources)
			clusterName, configID := compileAITestIdentity(t, resources)

			if got := clusterName != baselineCluster; got != tt.wantClusterChange {
				t.Errorf("OpenAI model cluster change = %t, want %t; baseline = %q, current = %q", got, tt.wantClusterChange, baselineCluster, clusterName)
			}
			if got := configID != baselineConfigID; got != tt.wantConfigChange {
				t.Errorf("OpenAI config ID change = %t, want %t; baseline = %q, current = %q", got, tt.wantConfigChange, baselineConfigID, configID)
			}
		})
	}
}

func TestCompilerRejectsOpenAIAuthenticationWithoutAPIKey(t *testing.T) {
	upstream := newAIUpstream("model-upstream", "")
	upstream.Spec.Authentication = &gatewayv1.UpstreamAuthentication{}
	result := Compile(Resources{Upstreams: []*gatewayv1.Upstream{upstream}})
	if !result.HasErrors() {
		t.Fatal("Compile(OpenAI authentication without API key) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compile(OpenAI authentication without API key) diagnostics = %v, want Upstream %q reason %q",
			result.Diagnostics,
			upstream.Name,
			ReasonInvalidSpec,
		)
	}
}

func TestCompilerRejectsUnsafeOpenAIAPIKey(t *testing.T) {
	upstream := newAIUpstream("model-upstream", "secret\r\ninjected")
	result := Compile(Resources{Upstreams: []*gatewayv1.Upstream{upstream}})
	if !result.HasErrors() {
		t.Fatal("Compile(unsafe OpenAI API key) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compile(unsafe OpenAI API key) diagnostics = %v, want Upstream %q reason %q",
			result.Diagnostics,
			upstream.Name,
			ReasonInvalidSpec,
		)
	}
}

func TestCompilerRejectsAPIKeyOverPlaintextOpenAIUpstream(t *testing.T) {
	upstream := newAIUpstream("model-upstream", "sk-test-secret")
	upstream.Spec.TLS = nil
	result := Compile(Resources{Upstreams: []*gatewayv1.Upstream{upstream}})
	if !result.HasErrors() {
		t.Fatal("Compile(API key over plaintext) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compile(API key over plaintext) diagnostics = %v, want Upstream %q reason %q",
			result.Diagnostics,
			upstream.Name,
			ReasonInvalidSpec,
		)
	}
}

func TestCompilerReportsUncompiledModelUpstreamAsInvalidReference(t *testing.T) {
	resources := newAICompilerResources()
	resources.Upstreams[0].Spec.TLS.ServerName = "invalid server name"
	result := Compile(resources)
	if !result.HasErrors() {
		t.Fatal("Compile(uncompiled model upstream) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindUpstream, resources.Upstreams[0].Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compile(uncompiled model upstream) diagnostics = %v, want Upstream %q reason %q",
			result.Diagnostics,
			resources.Upstreams[0].Name,
			ReasonInvalidSpec,
		)
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindRoute, resources.Routes[0].Name, ReasonInvalidReference) {
		t.Errorf(
			"Compile(uncompiled model upstream) diagnostics = %v, want Route %q reason %q",
			result.Diagnostics,
			resources.Routes[0].Name,
			ReasonInvalidReference,
		)
	}
}

func TestCompilerRejectsOpenAIUpstreamInOrdinaryRoute(t *testing.T) {
	gateway := newTestGateway("gateway", gatewayv1.ProtocolHTTP, 8080, "api.example.com", "")
	upstream := newAIUpstream("model-upstream", "")
	route := &gatewayv1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route"},
		Spec: gatewayv1.RouteSpec{
			Enabled:    true,
			ParentRefs: []gatewayv1.ParentRef{{Name: gateway.Name}},
			Hostnames:  []string{"api.example.com"},
			Rules: []gatewayv1.RouteRule{
				{
					Name:         "chat",
					PathPrefix:   "/v1/chat/completions",
					Methods:      []string{"POST"},
					UpstreamRefs: []gatewayv1.UpstreamRef{{Name: upstream.Name, Weight: 100}},
				},
			},
		},
	}
	result := Compile(Resources{
		Gateways:  []*gatewayv1.Gateway{gateway},
		Routes:    []*gatewayv1.Route{route},
		Upstreams: []*gatewayv1.Upstream{upstream},
	})
	if !result.HasErrors() {
		t.Fatal("Compile(OpenAI upstream in ordinary route) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindRoute, route.Name, ReasonInvalidReference) {
		t.Errorf(
			"Compile(OpenAI upstream in ordinary route) diagnostics = %v, want Route %q reason %q",
			result.Diagnostics,
			route.Name,
			ReasonInvalidReference,
		)
	}
}

func TestCompilerRejectsAIManagedRequestHeaderModifier(t *testing.T) {
	resources := newAICompilerResources()
	resources.Routes[0].Spec.Rules[0].Filters = []gatewayv1.RouteFilter{
		{
			Type: gatewayv1.RouteFilterRequestHeaderModifier,
			RequestHeaderModifier: &gatewayv1.HeaderModifier{
				Set: []gatewayv1.HeaderValue{{Name: "Authorization", Value: "Bearer manual-secret"}},
			},
		},
	}
	result := Compile(resources)
	if !result.HasErrors() {
		t.Fatal("Compile(AI-managed authorization header) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindRoute, resources.Routes[0].Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compile(AI-managed authorization header) diagnostics = %v, want Route %q reason %q",
			result.Diagnostics,
			resources.Routes[0].Name,
			ReasonInvalidSpec,
		)
	}
}

func TestCompilerRejectsAIManagedContentTypeModifier(t *testing.T) {
	resources := newAICompilerResources()
	resources.Routes[0].Spec.Rules[0].Filters = []gatewayv1.RouteFilter{
		{
			Type: gatewayv1.RouteFilterRequestHeaderModifier,
			RequestHeaderModifier: &gatewayv1.HeaderModifier{
				Set: []gatewayv1.HeaderValue{{Name: "Content-Type", Value: "text/plain"}},
			},
		},
	}
	result := Compile(resources)
	if !result.HasErrors() {
		t.Fatal("Compile(AI-managed content type header) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindRoute, resources.Routes[0].Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compile(AI-managed content type header) diagnostics = %v, want Route %q reason %q",
			result.Diagnostics,
			resources.Routes[0].Name,
			ReasonInvalidSpec,
		)
	}
}

func TestCompilerRejectsInternalAIRouteHeaderMatch(t *testing.T) {
	resources := newAICompilerResources()
	resources.Routes[0].Spec.Rules[0].Headers = []gatewayv1.HeaderMatch{{Name: aiRouteHeader, Value: "forged"}}
	result := Compile(resources)
	if !result.HasErrors() {
		t.Fatal("Compile(internal AI route header match) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindRoute, resources.Routes[0].Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compile(internal AI route header match) diagnostics = %v, want Route %q reason %q",
			result.Diagnostics,
			resources.Routes[0].Name,
			ReasonInvalidSpec,
		)
	}
}

func TestCompilerRejectsUnsupportedAIPath(t *testing.T) {
	resources := newAICompilerResources()
	resources.Routes[0].Spec.Rules[0].PathPrefix = "/chat"
	result := Compile(resources)
	if !result.HasErrors() {
		t.Fatal("Compile(unsupported AI path) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindRoute, resources.Routes[0].Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compile(unsupported AI path) diagnostics = %v, want Route %q reason %q",
			result.Diagnostics,
			resources.Routes[0].Name,
			ReasonInvalidSpec,
		)
	}
}

func newAICompilerResources() Resources {
	aiGateway := newTestGateway("ai-gateway", gatewayv1.ProtocolHTTP, 8080, "ai.example.com", "")
	appGateway := newTestGateway("app-gateway", gatewayv1.ProtocolHTTP, 8081, "app.example.com", "")
	modelUpstream := newAIUpstream("model-upstream", "sk-test-secret")
	appUpstream := &gatewayv1.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "app-upstream"},
		Spec: gatewayv1.UpstreamSpec{
			Type:     gatewayv1.UpstreamTypeApplication,
			Protocol: gatewayv1.UpstreamProtocolHTTP,
			Endpoints: []gatewayv1.Endpoint{
				{Name: "primary", Address: "192.0.2.20", Port: 8080, Weight: 100, Enabled: true},
			},
		},
	}
	aiRoute := &gatewayv1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "ai-route"},
		Spec: gatewayv1.RouteSpec{
			Enabled:    true,
			ParentRefs: []gatewayv1.ParentRef{{Name: aiGateway.Name}},
			Hostnames:  []string{"ai.example.com"},
			Rules: []gatewayv1.RouteRule{
				{
					Name:       "chat",
					PathPrefix: "/v1/chat/completions",
					Methods:    []string{"POST"},
					ModelRouting: &gatewayv1.ModelRouting{
						Models: []gatewayv1.ModelRoute{
							{Model: "assistant", UpstreamRef: modelUpstream.Name},
						},
					},
				},
			},
		},
	}
	appRoute := &gatewayv1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "app-route"},
		Spec: gatewayv1.RouteSpec{
			Enabled:    true,
			ParentRefs: []gatewayv1.ParentRef{{Name: appGateway.Name}},
			Hostnames:  []string{"app.example.com"},
			Rules: []gatewayv1.RouteRule{
				{
					Name:         "api",
					PathPrefix:   "/api",
					Methods:      []string{"GET"},
					UpstreamRefs: []gatewayv1.UpstreamRef{{Name: appUpstream.Name, Weight: 100}},
				},
			},
		},
	}
	return Resources{
		Gateways:  []*gatewayv1.Gateway{aiGateway, appGateway},
		Routes:    []*gatewayv1.Route{aiRoute, appRoute},
		Upstreams: []*gatewayv1.Upstream{modelUpstream, appUpstream},
	}
}

func newAIUpstream(name, apiKey string) *gatewayv1.Upstream {
	upstream := &gatewayv1.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1.UpstreamSpec{
			Type:     gatewayv1.UpstreamTypeModel,
			Protocol: gatewayv1.UpstreamProtocolOpenAI,
			TLS:      &gatewayv1.UpstreamTLS{ServerName: "api.example.com"},
			Model: &gatewayv1.ModelSpec{
				Provider:    gatewayv1.ModelProviderOpenAI,
				APIBasePath: "/v1",
				Models: []gatewayv1.ModelCatalogItem{
					{Name: "assistant", DisplayName: "Assistant", Enabled: true},
					{Name: "gpt-assistant", DisplayName: "GPT Assistant", Enabled: true},
					{Name: "gpt-reasoning", DisplayName: "GPT Reasoning", Enabled: true},
					{Name: "gpt-assistant-v2", DisplayName: "GPT Assistant v2", Enabled: true},
				},
			},
			Endpoints: []gatewayv1.Endpoint{
				{Name: "primary", Address: "192.0.2.10", Port: 8080, Weight: 100, Enabled: true},
			},
		},
	}
	if apiKey != "" {
		upstream.Spec.Authentication = &gatewayv1.UpstreamAuthentication{
			APIKey: &gatewayv1.APIKeyAuthentication{Value: apiKey},
		}
	}
	return upstream
}

func compileAITestIdentity(t *testing.T, resources Resources) (string, string) {
	t.Helper()
	result := Compile(resources)
	if result.HasErrors() {
		t.Fatalf("Compile(OpenAI config identity) diagnostics = %v, want no errors", result.Diagnostics)
	}
	config := decodeAIProxyConfig(t, findCompiledListener(t, result.Config.Listeners, "ingate/http-8080"))
	if got, want := len(config.Routes), 1; got != want {
		t.Fatalf("Compile(OpenAI config identity) AI route count = %d, want %d", got, want)
	}
	configID := config.Routes[0].ConfigID
	findCompiledRoute(t, result.Config.Routes, envoyAIRouteName("ai-gateway", "ai-route", "chat", "POST", configID))
	if got, want := len(config.Routes[0].Upstreams), 1; got != want {
		t.Fatalf("Compile(OpenAI config identity) upstream count = %d, want %d", got, want)
	}
	return config.Routes[0].Upstreams[0].Cluster, configID
}

func assertAIUpstream(
	t *testing.T,
	upstream pluginaiproxy.UpstreamConfig,
	protocol llm.Protocol,
	apiKeyHeader string,
	apiKeyPrefix string,
	wantHeaders map[string]string,
) {
	t.Helper()
	if upstream.ID == "" {
		t.Fatal("compiled AI upstream is missing")
	}
	if upstream.Protocol != protocol {
		t.Errorf("compiled AI upstream %q protocol = %q, want %q", upstream.ID, upstream.Protocol, protocol)
	}
	if upstream.APIKeyHeader != apiKeyHeader {
		t.Errorf("compiled AI upstream %q API key header = %q, want %q", upstream.ID, upstream.APIKeyHeader, apiKeyHeader)
	}
	if upstream.APIKeyPrefix != apiKeyPrefix {
		t.Errorf("compiled AI upstream %q API key prefix = %q, want %q", upstream.ID, upstream.APIKeyPrefix, apiKeyPrefix)
	}
	gotHeaders := make(map[string]string, len(upstream.Headers))
	for _, header := range upstream.Headers {
		gotHeaders[strings.ToLower(header.Name)] = header.Value
	}
	if len(gotHeaders) != len(wantHeaders) {
		t.Errorf("compiled AI upstream %q static headers = %v, want %v", upstream.ID, gotHeaders, wantHeaders)
		return
	}
	for name, value := range wantHeaders {
		if gotHeaders[name] != value {
			t.Errorf("compiled AI upstream %q static header %q = %q, want %q", upstream.ID, name, gotHeaders[name], value)
		}
	}
}

func findCompiledRoutes(t *testing.T, configs []*routev3.RouteConfiguration, name string) []*routev3.Route {
	t.Helper()
	var result []*routev3.Route
	for _, config := range configs {
		for _, virtualHost := range config.VirtualHosts {
			for _, route := range virtualHost.Routes {
				if route.Name == name {
					result = append(result, route)
				}
			}
		}
	}
	if len(result) == 0 {
		t.Fatalf("compiled routes %q not found", name)
	}
	return result
}

func findAIContinuationRoute(
	t *testing.T,
	routes []*routev3.Route,
	configID string,
	cluster string,
) *routev3.Route {
	t.Helper()
	for _, route := range routes {
		if exactHeaderMatch(route.GetMatch(), aiRouteHeader) == configID &&
			exactHeaderMatch(route.GetMatch(), aiClusterHeader) == cluster {
			return route
		}
	}
	t.Fatalf("compiled AI continuation route for config %q cluster %q not found", configID, cluster)
	return nil
}

func exactHeaderMatch(match *routev3.RouteMatch, name string) string {
	for _, header := range match.GetHeaders() {
		if strings.EqualFold(header.GetName(), name) {
			return header.GetExactMatch()
		}
	}
	return ""
}

func findRouteConfiguration(
	t *testing.T,
	configs []*routev3.RouteConfiguration,
	routeName string,
) *routev3.RouteConfiguration {
	t.Helper()
	for _, config := range configs {
		for _, virtualHost := range config.VirtualHosts {
			for _, route := range virtualHost.Routes {
				if route.Name == routeName {
					return config
				}
			}
		}
	}
	t.Fatalf("route configuration containing %q not found", routeName)
	return nil
}

func findCompiledRoute(t *testing.T, configs []*routev3.RouteConfiguration, name string) *routev3.Route {
	t.Helper()
	for _, config := range configs {
		for _, virtualHost := range config.VirtualHosts {
			for _, route := range virtualHost.Routes {
				if route.Name == name {
					return route
				}
			}
		}
	}
	t.Fatalf("compiled route %q not found", name)
	return nil
}

func findCompiledCluster(t *testing.T, clusters []*clusterv3.Cluster, name string) *clusterv3.Cluster {
	t.Helper()
	for _, cluster := range clusters {
		if cluster.Name == name {
			return cluster
		}
	}
	t.Fatalf("compiled cluster %q not found", name)
	return nil
}

func findCompiledEndpoint(t *testing.T, endpoints []*endpointv3.ClusterLoadAssignment, name string) *endpointv3.ClusterLoadAssignment {
	t.Helper()
	for _, endpoint := range endpoints {
		if endpoint.ClusterName == name {
			return endpoint
		}
	}
	t.Fatalf("compiled endpoint %q not found", name)
	return nil
}

func findCompiledListener(t *testing.T, listeners []*listenerv3.Listener, name string) *listenerv3.Listener {
	t.Helper()
	for _, listener := range listeners {
		if listener.Name == name {
			return listener
		}
	}
	t.Fatalf("compiled listener %q not found", name)
	return nil
}

func decodeHTTPConnectionManager(t *testing.T, listener *listenerv3.Listener) *hcmv3.HttpConnectionManager {
	t.Helper()
	if len(listener.FilterChains) != 1 || len(listener.FilterChains[0].Filters) != 1 {
		t.Fatalf("listener %q filter chains = %v, want one HTTP connection manager", listener.Name, listener.FilterChains)
	}
	manager := &hcmv3.HttpConnectionManager{}
	if err := listener.FilterChains[0].Filters[0].GetTypedConfig().UnmarshalTo(manager); err != nil {
		t.Fatalf("decode listener %q HTTP connection manager error = %v, want nil", listener.Name, err)
	}
	return manager
}

func decodeAIProxyConfig(t *testing.T, listener *listenerv3.Listener) pluginaiproxy.PluginConfig {
	t.Helper()
	manager := decodeHTTPConnectionManager(t, listener)
	for _, filter := range manager.HttpFilters {
		if filter.Name != aiProxyHTTPFilterName {
			continue
		}
		wasm := &httpwasmv3.Wasm{}
		if err := filter.GetTypedConfig().UnmarshalTo(wasm); err != nil {
			t.Fatalf("decode listener %q AI proxy filter error = %v, want nil", listener.Name, err)
		}
		if !wasm.GetConfig().GetAllowOnHeadersStopIteration().GetValue() {
			t.Fatalf("listener %q AI proxy filter does not allow Header pause followed by Body processing", listener.Name)
		}
		configuration := &wrapperspb.StringValue{}
		if err := wasm.GetConfig().GetConfiguration().UnmarshalTo(configuration); err != nil {
			t.Fatalf("decode listener %q AI proxy configuration error = %v, want nil", listener.Name, err)
		}
		config, err := pluginaiproxy.ParsePluginConfig([]byte(configuration.Value))
		if err != nil {
			t.Fatalf("ParsePluginConfig(listener %q) error = %v, want nil", listener.Name, err)
		}
		return config
	}
	t.Fatalf("listener %q does not contain AI proxy filter", listener.Name)
	return pluginaiproxy.PluginConfig{}
}
