package apiserver

import (
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// PluginSourceStore 读写 PluginSource 声明式资源。
type PluginSourceStore = resourceStore[
	resource.PluginSource,
	*resource.PluginSource,
	*resource.PluginSourceList,
	resource.PluginSourceSpec,
]

// WasmPluginStore 读写 WasmPlugin 声明式资源。
type WasmPluginStore = resourceStore[
	resource.WasmPlugin,
	*resource.WasmPlugin,
	*resource.WasmPluginList,
	resource.WasmPluginSpec,
]

// NewPluginSourceStore 创建 PluginSource Store。
func NewPluginSourceStore(client clientset.Interface) *PluginSourceStore {
	return &PluginSourceStore{
		kind:     "plugin source",
		listKind: "plugin sources",
		client:   client.GatewayV1().PluginSources(),
		items: func(list *resource.PluginSourceList) ([]resource.PluginSource, string) {
			return list.Items, list.Continue
		},
		newObject: func(resourceID string, spec resource.PluginSourceSpec) *resource.PluginSource {
			return newResource(
				resourceID,
				resource.KindPluginSource,
				&resource.PluginSource{Spec: spec},
			)
		},
		setSpec: func(object *resource.PluginSource, spec resource.PluginSourceSpec) {
			object.Spec = spec
		},
	}
}

// NewWasmPluginStore 创建 WasmPlugin Store。
func NewWasmPluginStore(client clientset.Interface) *WasmPluginStore {
	return &WasmPluginStore{
		kind:     "Wasm plugin",
		listKind: "Wasm plugins",
		client:   client.GatewayV1().WasmPlugins(),
		items: func(list *resource.WasmPluginList) ([]resource.WasmPlugin, string) {
			return list.Items, list.Continue
		},
		newObject: func(resourceID string, spec resource.WasmPluginSpec) *resource.WasmPlugin {
			return newResource(
				resourceID,
				resource.KindWasmPlugin,
				&resource.WasmPlugin{Spec: spec},
			)
		},
		setSpec: func(object *resource.WasmPlugin, spec resource.WasmPluginSpec) {
			object.Spec = spec
		},
	}
}
