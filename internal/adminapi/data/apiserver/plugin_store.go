package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// PluginSourceStore 读写 PluginSource 声明式资源。
type PluginSourceStore struct {
	*resourceStore[resource.PluginSource, *resource.PluginSource, resource.PluginSourceSpec]
}

// WasmPluginStore 读写 WasmPlugin 声明式资源。
type WasmPluginStore struct {
	*resourceStore[resource.WasmPlugin, *resource.WasmPlugin, resource.WasmPluginSpec]
}

// NewPluginSourceStore 创建 PluginSource Store。
func NewPluginSourceStore(client clientset.Interface) *PluginSourceStore {
	return &PluginSourceStore{resourceStore: newResourceStore(
		"plugin source",
		"plugin sources",
		func() createResourceClient[*resource.PluginSource] {
			return client.GatewayV1().PluginSources()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.PluginSource, string, error) {
			resources := client.GatewayV1().PluginSources()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
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
	)}
}

// NewWasmPluginStore 创建 WasmPlugin Store。
func NewWasmPluginStore(client clientset.Interface) *WasmPluginStore {
	return &WasmPluginStore{resourceStore: newResourceStore(
		"Wasm plugin",
		"Wasm plugins",
		func() createResourceClient[*resource.WasmPlugin] {
			return client.GatewayV1().WasmPlugins()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.WasmPlugin, string, error) {
			resources := client.GatewayV1().WasmPlugins()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
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
	)}
}
