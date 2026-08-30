package apiserver

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *WasmPluginStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.WasmPlugin,
	spec resource.WasmPluginSpec,
) (*resource.WasmPlugin, error) {
	pluginID := observed.Name
	updated, err := updateResource(
		ctx,
		s.client.GatewayV1().WasmPlugins(),
		observed,
		func(plugin *resource.WasmPlugin) { plugin.Spec = spec },
	)
	if apierrors.IsConflict(err) {
		return nil, fmt.Errorf(
			"replace Wasm plugin %q after conflict retries: %w",
			pluginID,
			err,
		)
	}
	return updated, resourceError("replace", "Wasm plugin", pluginID, err)
}

// Delete 删除 WasmPlugin。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *WasmPluginStore) Delete(
	ctx context.Context,
	observed *resource.WasmPlugin,
) error {
	pluginID := observed.Name
	err := deleteResource(ctx, s.client.GatewayV1().WasmPlugins(), observed)
	if apierrors.IsConflict(err) {
		return fmt.Errorf(
			"delete Wasm plugin %q after conflict retries: %w",
			pluginID,
			err,
		)
	}
	return resourceError("delete", "Wasm plugin", pluginID, err)
}
