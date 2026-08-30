package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// UpstreamStore 读写 Upstream 声明式资源。
type UpstreamStore struct {
	client clientset.Interface
}

// NewUpstreamStore 创建 Upstream Store。
func NewUpstreamStore(client clientset.Interface) *UpstreamStore {
	return &UpstreamStore{client: client}
}

// ListPage 分页返回 Upstream。
func (s *UpstreamStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.Upstream], error) {
	upstreams, err := s.client.GatewayV1().Upstreams().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.Upstream]{}, listError("upstreams", err)
	}
	return biz.PageResult[resource.Upstream]{
		Items:      upstreams.Items,
		NextCursor: upstreams.Continue,
	}, nil
}

// Get 返回指定 Upstream。
func (s *UpstreamStore) Get(ctx context.Context, upstreamID string) (*resource.Upstream, error) {
	upstream, err := s.client.GatewayV1().Upstreams().Get(ctx, upstreamID, metav1.GetOptions{})
	return upstream, resourceError("get", "upstream", upstreamID, err)
}

// ListByIDs 返回当前存在的指定 Upstream。
func (s *UpstreamStore) ListByIDs(
	ctx context.Context,
	upstreamIDs []string,
) (map[string]*resource.Upstream, error) {
	return listByIDs(ctx, upstreamIDs, s.Get)
}

// Create 创建 Upstream。
func (s *UpstreamStore) Create(
	ctx context.Context,
	upstreamID string,
	spec resource.UpstreamSpec,
) (*resource.Upstream, error) {
	upstream := &resource.Upstream{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindUpstream),
		},
		ObjectMeta: metav1.ObjectMeta{Name: upstreamID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().Upstreams().Create(ctx, upstream, metav1.CreateOptions{})
	return created, resourceError("create", "upstream", upstreamID, err)
}

// ReplaceSpec 完整替换 Upstream 配置。
func (s *UpstreamStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.Upstream,
	spec resource.UpstreamSpec,
) (*resource.Upstream, error) {
	return replaceResourceSpec(
		ctx,
		s.client.GatewayV1().Upstreams(),
		"upstream",
		observed,
		func(upstream *resource.Upstream) { upstream.Spec = spec },
	)
}

// Delete 删除 Upstream。
func (s *UpstreamStore) Delete(
	ctx context.Context,
	observed *resource.Upstream,
) error {
	return deleteResource(
		ctx,
		s.client.GatewayV1().Upstreams(),
		"upstream",
		observed,
	)
}
