package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// WasmPluginStore 读写 WasmPlugin 声明式资源。
type WasmPluginStore struct {
	client clientset.Interface
}

// NewWasmPluginStore 创建 WasmPlugin Store。
func NewWasmPluginStore(client clientset.Interface) *WasmPluginStore {
	return &WasmPluginStore{client: client}
}

// ListPage 分页返回 WasmPlugin。
func (s *WasmPluginStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.WasmPlugin], error) {
	plugins, err := s.client.GatewayV1().WasmPlugins().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.WasmPlugin]{}, listError("Wasm plugins", err)
	}
	return biz.PageResult[resource.WasmPlugin]{
		Items:      plugins.Items,
		NextCursor: plugins.Continue,
	}, nil
}

// Get 返回指定的 WasmPlugin。
func (s *WasmPluginStore) Get(
	ctx context.Context,
	pluginID string,
) (*resource.WasmPlugin, error) {
	plugin, err := s.client.GatewayV1().WasmPlugins().Get(
		ctx,
		pluginID,
		metav1.GetOptions{},
	)
	return plugin, resourceError("get", "Wasm plugin", pluginID, err)
}

// Create 创建 WasmPlugin。
func (s *WasmPluginStore) Create(
	ctx context.Context,
	pluginID string,
	spec resource.WasmPluginSpec,
) (*resource.WasmPlugin, error) {
	plugin := &resource.WasmPlugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindWasmPlugin),
		},
		ObjectMeta: metav1.ObjectMeta{Name: pluginID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().WasmPlugins().Create(
		ctx,
		plugin,
		metav1.CreateOptions{},
	)
	return created, resourceError("create", "Wasm plugin", pluginID, err)
}

// ReplaceSpec 完整替换 WasmPlugin 配置。
func (s *WasmPluginStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.WasmPlugin,
	spec resource.WasmPluginSpec,
) (*resource.WasmPlugin, error) {
	return replaceResourceSpec(
		ctx,
		s.client.GatewayV1().WasmPlugins(),
		"Wasm plugin",
		observed,
		func(plugin *resource.WasmPlugin) { plugin.Spec = spec },
	)
}

// Delete 删除 WasmPlugin。
func (s *WasmPluginStore) Delete(
	ctx context.Context,
	observed *resource.WasmPlugin,
) error {
	return deleteResource(
		ctx,
		s.client.GatewayV1().WasmPlugins(),
		"Wasm plugin",
		observed,
	)
}
