package apiserver

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	resources := client.GatewayV1().PluginSources()
	return newResourceStore(
		"plugin source",
		"plugin sources",
		resources,
		func(list *resource.PluginSourceList) ([]resource.PluginSource, string) {
			return list.Items, list.Continue
		},
		func(resourceID string, spec resource.PluginSourceSpec) *resource.PluginSource {
			return &resource.PluginSource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindPluginSource),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.PluginSource, spec resource.PluginSourceSpec) { object.Spec = spec },
	)
}

// NewWasmPluginStore 创建 WasmPlugin Store。
func NewWasmPluginStore(client clientset.Interface) *WasmPluginStore {
	resources := client.GatewayV1().WasmPlugins()
	return newResourceStore(
		"Wasm plugin",
		"Wasm plugins",
		resources,
		func(list *resource.WasmPluginList) ([]resource.WasmPlugin, string) {
			return list.Items, list.Continue
		},
		func(resourceID string, spec resource.WasmPluginSpec) *resource.WasmPlugin {
			return &resource.WasmPlugin{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindWasmPlugin),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.WasmPlugin, spec resource.WasmPluginSpec) { object.Spec = spec },
	)
}
