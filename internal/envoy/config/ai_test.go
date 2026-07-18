package config

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
	"google.golang.org/protobuf/types/known/wrapperspb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	pluginaiproxy "github.com/lgc202/ingate/pkg/plugin/aiproxy"
)

func TestCompilerBuildsOpenAIModelRoute(t *testing.T) {
	result := (Compiler{}).Compile(newAICompilerResources())
	if result.HasErrors() {
		t.Fatalf("Compiler.Compile(OpenAI model route) diagnostics = %v, want no errors", result.Diagnostics)
	}

	config := decodeAIProxyConfig(t, findCompiledListener(t, result.Config.Listeners, "ingate/http-8080"))
	if got, want := len(config.Routes), 1; got != want {
		t.Fatalf("Compiler.Compile(OpenAI model route) AI proxy route count = %d, want %d", got, want)
	}
	pluginRoute := config.Routes[0]
	if pluginRoute.ConfigID == "" {
		t.Fatal("Compiler.Compile(OpenAI model route) AI proxy config ID is empty")
	}

	routeName := runtimeAIRouteName("ai-gateway", "ai-route", "chat", "POST", pluginRoute.ConfigID)
	route := findCompiledRoute(t, result.Config.Routes, routeName)
	if got, want := route.GetMatch().GetPath(), openAIChatCompletionsPath; got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) exact path = %q, want %q", got, want)
	}
	action := route.GetRoute()
	if got := action.GetCluster(); !strings.HasPrefix(got, "model-upstream/ai/") {
		t.Errorf("Compiler.Compile(OpenAI model route) cluster = %q, want prefix %q", got, "model-upstream/ai/")
	}
	cluster := findCompiledCluster(t, result.Config.Clusters, action.GetCluster())
	if got, want := cluster.GetEdsClusterConfig().GetServiceName(), cluster.Name; got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) EDS service name = %q, want %q", got, want)
	}
	assignment := findCompiledEndpoint(t, result.Config.Endpoints, cluster.Name)
	if got, want := assignment.ClusterName, cluster.Name; got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) EDS cluster name = %q, want %q", got, want)
	}
	if got, want := action.GetHostRewriteLiteral(), "api.example.com"; got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) host rewrite = %q, want %q", got, want)
	}
	if action.GetTimeout() == nil {
		t.Fatal("Compiler.Compile(OpenAI model route) timeout = nil, want explicit zero timeout")
	}
	if got, want := action.GetTimeout().AsDuration(), time.Duration(0); got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) timeout = %v, want %v", got, want)
	}

	if got, want := pluginRoute.GatewayName, "ai-gateway"; got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) AI proxy gateway = %q, want %q", got, want)
	}
	if got, want := pluginRoute.RouteName, "ai-route"; got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) AI proxy route = %q, want %q", got, want)
	}
	if got, want := pluginRoute.RuleName, "chat"; got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) AI proxy rule = %q, want %q", got, want)
	}
	if got, want := pluginRoute.APIKey, "sk-test-secret"; got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) API key = %q, want %q", got, want)
	}
	if got, want := len(pluginRoute.Models), 1; got != want {
		t.Fatalf("Compiler.Compile(OpenAI model route) AI proxy model count = %d, want %d", got, want)
	}
	model := pluginRoute.Models[0]
	if got, want := model.Model, "assistant"; got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) client model = %q, want %q", got, want)
	}
	if got, want := model.UpstreamModel, "assistant"; got != want {
		t.Errorf("Compiler.Compile(OpenAI model route) default upstream model = %q, want %q", got, want)
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
	result := (Compiler{}).Compile(newAICompilerResources())
	if result.HasErrors() {
		t.Fatalf("Compiler.Compile(mixed AI and HTTP listeners) diagnostics = %v, want no errors", result.Diagnostics)
	}

	tests := []struct {
		listenerName string
		wantFilters  []string
		wantRoutes   int
	}{
		{
			listenerName: "ingate/http-8080",
			wantFilters:  []string{aiProxyHTTPFilterName, httpRouterFilterName},
			wantRoutes:   1,
		},
		{
			listenerName: "ingate/http-8081",
			wantFilters:  []string{aiProxyHTTPFilterName, httpRouterFilterName},
		},
	}
	for _, tt := range tests {
		t.Run(tt.listenerName, func(t *testing.T) {
			listener := findCompiledListener(t, result.Config.Listeners, tt.listenerName)
			manager := decodeHTTPConnectionManager(t, listener)
			gotFilters := make([]string, 0, len(manager.HttpFilters))
			for _, filter := range manager.HttpFilters {
				gotFilters = append(gotFilters, filter.Name)
			}
			if !slices.Equal(gotFilters, tt.wantFilters) {
				t.Errorf("Compiler.Compile(mixed AI and HTTP listeners) filters for %q = %v, want %v", tt.listenerName, gotFilters, tt.wantFilters)
			}
			config := decodeAIProxyConfig(t, listener)
			if got := len(config.Routes); got != tt.wantRoutes {
				t.Errorf("Compiler.Compile(mixed AI and HTTP listeners) AI route count for %q = %d, want %d", tt.listenerName, got, tt.wantRoutes)
			}
		})
	}
}

func TestCompilerVersionsOpenAIRuntimeConfig(t *testing.T) {
	newResources := func() ResourceSet {
		resources := newAICompilerResources()
		resources.Routes[0].Spec.Rules[0].ModelRouting.Models = []gatewayv1.ModelRoute{
			{Model: "assistant", UpstreamModel: "gpt-assistant"},
			{Model: "reasoning", UpstreamModel: "gpt-reasoning"},
		}
		return resources
	}

	baselineCluster, baselineConfigID := compileAITestIdentity(t, newResources())
	tests := []struct {
		name              string
		mutate            func(ResourceSet)
		wantClusterChange bool
		wantConfigChange  bool
	}{
		{
			name: "API key change",
			mutate: func(resources ResourceSet) {
				resources.Upstreams[0].Spec.Authentication.APIKey.Value = "sk-rotated-secret"
			},
			wantClusterChange: true,
			wantConfigChange:  true,
		},
		{
			name: "enabled endpoint change",
			mutate: func(resources ResourceSet) {
				resources.Upstreams[0].Spec.Endpoints[0].Address = "192.0.2.11"
			},
			wantClusterChange: true,
			wantConfigChange:  true,
		},
		{
			name: "model mapping change",
			mutate: func(resources ResourceSet) {
				resources.Routes[0].Spec.Rules[0].ModelRouting.Models[0].UpstreamModel = "gpt-assistant-v2"
			},
			wantConfigChange: true,
		},
		{
			name: "model order change",
			mutate: func(resources ResourceSet) {
				models := resources.Routes[0].Spec.Rules[0].ModelRouting.Models
				models[0], models[1] = models[1], models[0]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := newResources()
			tt.mutate(resources)
			clusterName, configID := compileAITestIdentity(t, resources)

			if got := clusterName != baselineCluster; got != tt.wantClusterChange {
				t.Errorf("OpenAI runtime cluster change = %t, want %t; baseline = %q, current = %q", got, tt.wantClusterChange, baselineCluster, clusterName)
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
	result := (Compiler{}).Compile(ResourceSet{Upstreams: []*gatewayv1.Upstream{upstream}})
	if !result.HasErrors() {
		t.Fatal("Compiler.Compile(OpenAI authentication without API key) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compiler.Compile(OpenAI authentication without API key) diagnostics = %v, want Upstream %q reason %q",
			result.Diagnostics,
			upstream.Name,
			ReasonInvalidSpec,
		)
	}
}

func TestCompilerRejectsUnsafeOpenAIAPIKey(t *testing.T) {
	upstream := newAIUpstream("model-upstream", "secret\r\ninjected")
	result := (Compiler{}).Compile(ResourceSet{Upstreams: []*gatewayv1.Upstream{upstream}})
	if !result.HasErrors() {
		t.Fatal("Compiler.Compile(unsafe OpenAI API key) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compiler.Compile(unsafe OpenAI API key) diagnostics = %v, want Upstream %q reason %q",
			result.Diagnostics,
			upstream.Name,
			ReasonInvalidSpec,
		)
	}
}

func TestCompilerRejectsAPIKeyOverPlaintextOpenAIUpstream(t *testing.T) {
	upstream := newAIUpstream("model-upstream", "sk-test-secret")
	upstream.Spec.TLS = nil
	result := (Compiler{}).Compile(ResourceSet{Upstreams: []*gatewayv1.Upstream{upstream}})
	if !result.HasErrors() {
		t.Fatal("Compiler.Compile(API key over plaintext) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compiler.Compile(API key over plaintext) diagnostics = %v, want Upstream %q reason %q",
			result.Diagnostics,
			upstream.Name,
			ReasonInvalidSpec,
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
	result := (Compiler{}).Compile(ResourceSet{
		Gateways:  []*gatewayv1.Gateway{gateway},
		Routes:    []*gatewayv1.Route{route},
		Upstreams: []*gatewayv1.Upstream{upstream},
	})
	if !result.HasErrors() {
		t.Fatal("Compiler.Compile(OpenAI upstream in ordinary route) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindRoute, route.Name, ReasonInvalidReference) {
		t.Errorf(
			"Compiler.Compile(OpenAI upstream in ordinary route) diagnostics = %v, want Route %q reason %q",
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
	result := (Compiler{}).Compile(resources)
	if !result.HasErrors() {
		t.Fatal("Compiler.Compile(AI-managed authorization header) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindRoute, resources.Routes[0].Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compiler.Compile(AI-managed authorization header) diagnostics = %v, want Route %q reason %q",
			result.Diagnostics,
			resources.Routes[0].Name,
			ReasonInvalidSpec,
		)
	}
}

func TestCompilerRejectsUnsupportedAIPath(t *testing.T) {
	resources := newAICompilerResources()
	resources.Routes[0].Spec.Rules[0].PathPrefix = "/chat"
	result := (Compiler{}).Compile(resources)
	if !result.HasErrors() {
		t.Fatal("Compiler.Compile(unsupported AI path) has errors = false, want true")
	}
	if !containsDiagnostic(result.Diagnostics, gatewayv1.KindRoute, resources.Routes[0].Name, ReasonInvalidSpec) {
		t.Errorf(
			"Compiler.Compile(unsupported AI path) diagnostics = %v, want Route %q reason %q",
			result.Diagnostics,
			resources.Routes[0].Name,
			ReasonInvalidSpec,
		)
	}
}

func newAICompilerResources() ResourceSet {
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
						UpstreamRef: modelUpstream.Name,
						Models: []gatewayv1.ModelRoute{
							{Model: "assistant"},
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
	return ResourceSet{
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

func compileAITestIdentity(t *testing.T, resources ResourceSet) (string, string) {
	t.Helper()
	result := (Compiler{}).Compile(resources)
	if result.HasErrors() {
		t.Fatalf("Compiler.Compile(OpenAI runtime identity) diagnostics = %v, want no errors", result.Diagnostics)
	}
	config := decodeAIProxyConfig(t, findCompiledListener(t, result.Config.Listeners, "ingate/http-8080"))
	if got, want := len(config.Routes), 1; got != want {
		t.Fatalf("Compiler.Compile(OpenAI runtime identity) AI route count = %d, want %d", got, want)
	}
	configID := config.Routes[0].ConfigID
	route := findCompiledRoute(t, result.Config.Routes, runtimeAIRouteName("ai-gateway", "ai-route", "chat", "POST", configID))
	return route.GetRoute().GetCluster(), configID
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
