package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// GatewayRepository 读写 Gateway 声明式资源
type GatewayRepository struct {
	client clientset.Interface
}

// NewGatewayRepository 创建 Gateway Repository
func NewGatewayRepository(client clientset.Interface) *GatewayRepository {
	return &GatewayRepository{client: client}
}

// ListPage 分页查询 Gateway 列表
func (r *GatewayRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Gateway], error) {
	gateways, err := r.client.GatewayV1().Gateways().List(ctx, pageOptions(page))
	if err != nil {
		return biz.PageResult[resource.Gateway]{}, pageError("gateways", err)
	}
	return biz.PageResult[resource.Gateway]{Items: gateways.Items, NextToken: gateways.Continue}, nil
}

// Get 查询单个 Gateway
func (r *GatewayRepository) Get(ctx context.Context, name string) (*resource.Gateway, error) {
	gateway, err := r.client.GatewayV1().Gateways().Get(ctx, name, metav1.GetOptions{})
	return gateway, resourceError("get", "gateway", name, err)
}

// Create 创建 Gateway
func (r *GatewayRepository) Create(ctx context.Context, id string, spec resource.GatewaySpec) error {
	gateway := &resource.Gateway{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindGateway)},
		ObjectMeta: metav1.ObjectMeta{Name: id},
		Spec:       spec,
	}
	_, err := r.client.GatewayV1().Gateways().Create(ctx, gateway, metav1.CreateOptions{})
	return resourceError("create", "gateway", id, err)
}

// Update 更新 Gateway，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *GatewayRepository) Update(ctx context.Context, id string, generation int64, spec resource.GatewaySpec) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Gateways().Get(ctx, id, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		_, err = r.client.GatewayV1().Gateways().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return resourceError("update", "gateway", id, err)
}

// Delete 删除 Gateway
func (r *GatewayRepository) Delete(ctx context.Context, name string) error {
	err := r.client.GatewayV1().Gateways().Delete(ctx, name, metav1.DeleteOptions{})
	return resourceError("delete", "gateway", name, err)
}
