package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// PluginSourceRepository 读写 PluginSource 声明式资源
type PluginSourceRepository struct {
	client clientset.Interface
}

// NewPluginSourceRepository 创建 PluginSource Repository
func NewPluginSourceRepository(client clientset.Interface) *PluginSourceRepository {
	return &PluginSourceRepository{client: client}
}

// ListPage 分页查询 PluginSource 列表
func (r *PluginSourceRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.PluginSource], error) {
	sources, err := r.client.GatewayV1().PluginSources().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.PluginSource]{}, listError("plugin sources", err)
	}
	return biz.PageResult[resource.PluginSource]{Items: sources.Items, NextCursor: sources.Continue}, nil
}

// Get 查询单个 PluginSource
func (r *PluginSourceRepository) Get(ctx context.Context, name string) (*resource.PluginSource, error) {
	source, err := r.client.GatewayV1().PluginSources().Get(ctx, name, metav1.GetOptions{})
	return source, resourceError("get", "plugin source", name, err)
}

// Create 创建 PluginSource
func (r *PluginSourceRepository) Create(
	ctx context.Context,
	name string,
	spec resource.PluginSourceSpec,
) (*resource.PluginSource, error) {
	source := &resource.PluginSource{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindPluginSource)},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	created, err := r.client.GatewayV1().PluginSources().Create(ctx, source, metav1.CreateOptions{})
	return created, resourceError("create", "plugin source", name, err)
}

// Update 更新 PluginSource，并只重试 status 写入导致的 ResourceVersion 冲突
func (r *PluginSourceRepository) Update(
	ctx context.Context,
	name string,
	generation int64,
	spec resource.PluginSourceSpec,
) (*resource.PluginSource, error) {
	var updated *resource.PluginSource
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().PluginSources().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		updated, err = r.client.GatewayV1().PluginSources().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return updated, resourceError("update", "plugin source", name, err)
}

// Delete 删除 PluginSource
func (r *PluginSourceRepository) Delete(ctx context.Context, name string, generation int64) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().PluginSources().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		resourceVersion := current.ResourceVersion
		return r.client.GatewayV1().PluginSources().Delete(ctx, name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &resourceVersion},
		})
	})
	return resourceError("delete", "plugin source", name, err)
}
