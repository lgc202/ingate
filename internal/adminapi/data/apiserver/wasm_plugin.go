package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// WasmPluginRepository 读写 WasmPlugin 声明式资源
type WasmPluginRepository struct {
	client clientset.Interface
}

// NewWasmPluginRepository 创建 WasmPlugin Repository
func NewWasmPluginRepository(client clientset.Interface) *WasmPluginRepository {
	return &WasmPluginRepository{client: client}
}

// ListPage 分页查询 WasmPlugin 列表
func (r *WasmPluginRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.WasmPlugin], error) {
	plugins, err := r.client.GatewayV1().WasmPlugins().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.WasmPlugin]{}, listError("Wasm plugins", err)
	}
	return biz.PageResult[resource.WasmPlugin]{Items: plugins.Items, NextCursor: plugins.Continue}, nil
}

// Get 查询单个 WasmPlugin
func (r *WasmPluginRepository) Get(ctx context.Context, name string) (*resource.WasmPlugin, error) {
	plugin, err := r.client.GatewayV1().WasmPlugins().Get(ctx, name, metav1.GetOptions{})
	return plugin, resourceError("get", "Wasm plugin", name, err)
}

// Create 创建 WasmPlugin
func (r *WasmPluginRepository) Create(
	ctx context.Context,
	name string,
	spec resource.WasmPluginSpec,
) (*resource.WasmPlugin, error) {
	plugin := &resource.WasmPlugin{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindWasmPlugin)},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	created, err := r.client.GatewayV1().WasmPlugins().Create(ctx, plugin, metav1.CreateOptions{})
	return created, resourceError("create", "Wasm plugin", name, err)
}

// Update 更新 WasmPlugin，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *WasmPluginRepository) Update(
	ctx context.Context,
	name string,
	generation int64,
	spec resource.WasmPluginSpec,
) (*resource.WasmPlugin, error) {
	var updated *resource.WasmPlugin
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().WasmPlugins().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		updated, err = r.client.GatewayV1().WasmPlugins().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return updated, resourceError("update", "Wasm plugin", name, err)
}

// Delete 删除 WasmPlugin
func (r *WasmPluginRepository) Delete(ctx context.Context, name string, generation int64) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().WasmPlugins().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		resourceVersion := current.ResourceVersion
		return r.client.GatewayV1().WasmPlugins().Delete(ctx, name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &resourceVersion},
		})
	})
	return resourceError("delete", "Wasm plugin", name, err)
}
