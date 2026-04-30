package server

import (
	"encoding/json"
	"testing"

	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	httpwasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	wasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestResponseBuilderBuildListenersWithWasmPlugin(t *testing.T) {
	pluginConfigRaw := json.RawMessage(`{"_rules_":[{"_match_route_":["chat"],"provider":{"type":"OpenAICompatible"}}]}`)
	configs := []snapshotConfig{
		{
			Gateway: "public",
			Version: "xds/public",
			Config: targetxds.Config{
				Listeners: []targetxds.Listener{
					{
						Name:            "public/http",
						Port:            80,
						RouteConfigName: "public/http/routes",
					},
				},
				Plugins: []targetxds.Plugin{
					{
						Name:    "ai-proxy",
						Runtime: resource.PluginRuntimeWasm,
						Image:   "/var/lib/ingate/plugins/ai-proxy.wasm",
					},
				},
				PluginBindings: []targetxds.PluginBinding{
					{
						Name: "chat-ai-proxy",
						Target: targetxds.PluginTarget{
							Kind: resource.KindGateway,
							Name: "public",
						},
						Priority:      100,
						FailurePolicy: resource.PluginFailurePolicyFailClose,
						Plugins: []targetxds.PluginRef{
							{
								Name:   "ai-proxy",
								Config: pluginConfigRaw,
							},
						},
					},
				},
			},
		},
	}

	resources, err := (responseBuilder{}).buildListeners(configs)
	if err != nil {
		t.Fatalf("buildListeners() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("len(resources) = %d, want 1", len(resources))
	}

	var listener listenerv3.Listener
	if err := resources[0].UnmarshalTo(&listener); err != nil {
		t.Fatalf("UnmarshalTo(listener) error = %v", err)
	}
	if len(listener.FilterChains) != 1 {
		t.Fatalf("len(FilterChains) = %d, want 1", len(listener.FilterChains))
	}
	filters := listener.FilterChains[0].Filters
	if len(filters) != 1 {
		t.Fatalf("len(Filters) = %d, want 1", len(filters))
	}

	var hcm hcmv3.HttpConnectionManager
	if err := filters[0].GetTypedConfig().UnmarshalTo(&hcm); err != nil {
		t.Fatalf("UnmarshalTo(hcm) error = %v", err)
	}
	if len(hcm.HttpFilters) != 2 {
		t.Fatalf("len(HttpFilters) = %d, want 2", len(hcm.HttpFilters))
	}
	if hcm.HttpFilters[0].Name != httpWasmFilterName {
		t.Fatalf("HttpFilters[0].Name = %q, want %q", hcm.HttpFilters[0].Name, httpWasmFilterName)
	}
	if hcm.HttpFilters[1].Name != httpRouterFilterName {
		t.Fatalf("HttpFilters[1].Name = %q, want %q", hcm.HttpFilters[1].Name, httpRouterFilterName)
	}

	var wasm httpwasmv3.Wasm
	if err := hcm.HttpFilters[0].GetTypedConfig().UnmarshalTo(&wasm); err != nil {
		t.Fatalf("UnmarshalTo(wasm) error = %v", err)
	}
	pluginConfig := wasm.GetConfig()
	if pluginConfig.GetName() != "ai-proxy" {
		t.Fatalf("PluginConfig.Name = %q, want ai-proxy", pluginConfig.GetName())
	}
	if pluginConfig.GetFailurePolicy() != wasmv3.FailurePolicy_FAIL_CLOSED {
		t.Fatalf("PluginConfig.FailurePolicy = %v, want fail close", pluginConfig.GetFailurePolicy())
	}
	if pluginConfig.GetVmConfig().GetCode().GetLocal().GetFilename() != "/var/lib/ingate/plugins/ai-proxy.wasm" {
		t.Fatalf("PluginConfig code filename = %q", pluginConfig.GetVmConfig().GetCode().GetLocal().GetFilename())
	}

	var pluginJSON wrapperspb.StringValue
	if err := pluginConfig.GetConfiguration().UnmarshalTo(&pluginJSON); err != nil {
		t.Fatalf("UnmarshalTo(plugin config) error = %v", err)
	}
	if pluginJSON.Value != string(pluginConfigRaw) {
		t.Fatalf("Plugin configuration = %q, want provider config", pluginJSON.Value)
	}
}
