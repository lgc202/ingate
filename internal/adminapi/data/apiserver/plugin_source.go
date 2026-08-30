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

// PluginSourceStore 读写 PluginSource 声明式资源。
type PluginSourceStore struct {
	client clientset.Interface
}

// NewPluginSourceStore 创建 PluginSource Store。
func NewPluginSourceStore(client clientset.Interface) *PluginSourceStore {
	return &PluginSourceStore{client: client}
}

// ListPage 分页返回自定义 PluginSource。
func (s *PluginSourceStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.PluginSource], error) {
	sources, err := s.client.GatewayV1().PluginSources().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.PluginSource]{}, listError("plugin sources", err)
	}
	return biz.PageResult[resource.PluginSource]{
		Items:      sources.Items,
		NextCursor: sources.Continue,
	}, nil
}

// Get 返回指定的 PluginSource。
func (s *PluginSourceStore) Get(
	ctx context.Context,
	sourceID string,
) (*resource.PluginSource, error) {
	source, err := s.client.GatewayV1().PluginSources().Get(
		ctx,
		sourceID,
		metav1.GetOptions{},
	)
	return source, resourceError("get", "plugin source", sourceID, err)
}

// Create 创建 PluginSource。
func (s *PluginSourceStore) Create(
	ctx context.Context,
	sourceID string,
	spec resource.PluginSourceSpec,
) (*resource.PluginSource, error) {
	source := &resource.PluginSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindPluginSource),
		},
		ObjectMeta: metav1.ObjectMeta{Name: sourceID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().PluginSources().Create(
		ctx,
		source,
		metav1.CreateOptions{},
	)
	return created, resourceError("create", "plugin source", sourceID, err)
}

// ReplaceSpec 完整替换 PluginSource 配置。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *PluginSourceStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.PluginSource,
	spec resource.PluginSourceSpec,
) (*resource.PluginSource, error) {
	sourceID := observed.Name
	updated, err := updateResource(
		ctx,
		s.client.GatewayV1().PluginSources(),
		observed,
		func(source *resource.PluginSource) { source.Spec = spec },
	)
	if apierrors.IsConflict(err) {
		return nil, fmt.Errorf(
			"replace plugin source %q after conflict retries: %w",
			sourceID,
			err,
		)
	}
	return updated, resourceError("replace", "plugin source", sourceID, err)
}

// Delete 删除 PluginSource。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *PluginSourceStore) Delete(
	ctx context.Context,
	observed *resource.PluginSource,
) error {
	sourceID := observed.Name
	err := deleteResource(ctx, s.client.GatewayV1().PluginSources(), observed)
	if apierrors.IsConflict(err) {
		return fmt.Errorf(
			"delete plugin source %q after conflict retries: %w",
			sourceID,
			err,
		)
	}
	return resourceError("delete", "plugin source", sourceID, err)
}
