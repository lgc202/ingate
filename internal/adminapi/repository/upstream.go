package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Upstream 将写入交给 API Server，将查询交给 MySQL 投影。
type Upstream struct {
	client gatewayclient.UpstreamInterface
	store  *mysql.Store
}

// NewUpstream 创建 Upstream Repository。
func NewUpstream(client gatewayclient.UpstreamInterface, store *mysql.Store) *Upstream {
	return &Upstream{client: client, store: store}
}

// Create 创建 Upstream，并返回 API Server 保存后的业务状态。
func (r *Upstream) Create(ctx context.Context, upstream domain.Upstream) (domain.Upstream, error) {
	created, err := r.client.Create(ctx, upstreamToAPI(upstream), metav1.CreateOptions{})
	if err != nil {
		return domain.Upstream{}, translateResourceError(err)
	}

	return upstreamFromAPI(created), nil
}

// Get 从 MySQL 查询模型读取指定 Upstream。
func (r *Upstream) Get(ctx context.Context, name string) (domain.Upstream, error) {
	upstream, err := r.store.GetUpstream(ctx, name)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return domain.Upstream{}, fmt.Errorf("%w: %w", ErrNotFound, err)
		}
		return domain.Upstream{}, err
	}

	return upstreamFromMySQL(upstream), nil
}

// List 从 MySQL 查询模型按页码读取 Upstream。
func (r *Upstream) List(ctx context.Context, size, number int32) ([]domain.Upstream, int64, error) {
	result, err := r.store.ListUpstreams(ctx, size, number)
	if err != nil {
		return nil, 0, err
	}

	upstreams := make([]domain.Upstream, len(result.Items))
	for i, upstream := range result.Items {
		upstreams[i] = upstreamFromMySQL(upstream)
	}

	return upstreams, result.TotalCount, nil
}

// Update 使用 API Server 当前对象核对版本，并替换 Upstream 的可编辑配置。
func (r *Upstream) Update(ctx context.Context, upstream domain.Upstream) (domain.Upstream, error) {
	current, err := r.client.Get(ctx, upstream.Name, metav1.GetOptions{})
	if err != nil {
		return domain.Upstream{}, translateResourceError(err)
	}

	if current.ResourceVersion != upstream.Version {
		return domain.Upstream{}, ErrConflict
	}

	current.Spec = upstreamToAPI(upstream).Spec
	updated, err := r.client.Update(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		return domain.Upstream{}, translateResourceError(err)
	}

	return upstreamFromAPI(updated), nil
}

// Delete 删除指定 Upstream。
func (r *Upstream) Delete(ctx context.Context, name string) error {
	return translateResourceError(r.client.Delete(ctx, name, metav1.DeleteOptions{}))
}

func upstreamToAPI(upstream domain.Upstream) *gatewayv1alpha1.Upstream {
	endpoints := make([]gatewayv1alpha1.Endpoint, len(upstream.Endpoints))
	for i, endpoint := range upstream.Endpoints {
		endpoints[i] = gatewayv1alpha1.Endpoint{
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  endpoint.Weight,
		}
	}

	return &gatewayv1alpha1.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: upstream.Name},
		Spec:       gatewayv1alpha1.UpstreamSpec{Endpoints: endpoints},
	}
}

func upstreamFromAPI(upstream *gatewayv1alpha1.Upstream) domain.Upstream {
	endpoints := make([]domain.Endpoint, len(upstream.Spec.Endpoints))
	for i, endpoint := range upstream.Spec.Endpoints {
		endpoints[i] = domain.Endpoint{
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  endpoint.Weight,
		}
	}

	return domain.Upstream{
		Name:       upstream.Name,
		Endpoints:  endpoints,
		CreateTime: upstream.CreationTimestamp.Time,
		Version:    upstream.ResourceVersion,
	}
}

func upstreamFromMySQL(upstream mysql.Upstream) domain.Upstream {
	endpoints := make([]domain.Endpoint, len(upstream.Endpoints))
	for i, endpoint := range upstream.Endpoints {
		endpoints[i] = domain.Endpoint{
			Address: endpoint.Address,
			Port:    int32(endpoint.Port),
			Weight:  endpoint.Weight,
		}
	}

	return domain.Upstream{
		Name:       upstream.Name,
		Endpoints:  endpoints,
		CreateTime: upstream.CreatedAt,
		Version:    upstream.ResourceVersion,
	}
}
