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

// GatewayStore 读写 Gateway 声明式资源。
type GatewayStore struct {
	client clientset.Interface
}

// NewGatewayStore 创建 Gateway Store。
func NewGatewayStore(client clientset.Interface) *GatewayStore {
	return &GatewayStore{client: client}
}

// ListPage 分页返回 Gateway。
func (s *GatewayStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.Gateway], error) {
	gateways, err := s.client.GatewayV1().Gateways().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.Gateway]{}, listError("gateways", err)
	}
	return biz.PageResult[resource.Gateway]{
		Items:      gateways.Items,
		NextCursor: gateways.Continue,
	}, nil
}

// Get 返回指定 Gateway。
func (s *GatewayStore) Get(ctx context.Context, gatewayID string) (*resource.Gateway, error) {
	gateway, err := s.client.GatewayV1().Gateways().Get(ctx, gatewayID, metav1.GetOptions{})
	return gateway, resourceError("get", "gateway", gatewayID, err)
}

// ListByIDs 返回当前存在的指定 Gateway。
func (s *GatewayStore) ListByIDs(
	ctx context.Context,
	gatewayIDs []string,
) (map[string]*resource.Gateway, error) {
	return listByIDs(ctx, gatewayIDs, s.Get)
}

// Create 创建 Gateway。
func (s *GatewayStore) Create(
	ctx context.Context,
	gatewayID string,
	spec resource.GatewaySpec,
) (*resource.Gateway, error) {
	gateway := &resource.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindGateway),
		},
		ObjectMeta: metav1.ObjectMeta{Name: gatewayID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().Gateways().Create(ctx, gateway, metav1.CreateOptions{})
	return created, resourceError("create", "gateway", gatewayID, err)
}

// ReplaceSpec 完整替换 Gateway 配置。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *GatewayStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.Gateway,
	spec resource.GatewaySpec,
) (*resource.Gateway, error) {
	gatewayID := observed.Name
	updated, err := updateResource(
		ctx,
		s.client.GatewayV1().Gateways(),
		observed,
		func(gateway *resource.Gateway) { gateway.Spec = spec },
	)
	if apierrors.IsConflict(err) {
		return nil, fmt.Errorf("replace gateway %q after conflict retries: %w", gatewayID, err)
	}
	return updated, resourceError("replace", "gateway", gatewayID, err)
}

// Delete 删除 Gateway。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *GatewayStore) Delete(
	ctx context.Context,
	observed *resource.Gateway,
) error {
	gatewayID := observed.Name
	err := deleteResource(ctx, s.client.GatewayV1().Gateways(), observed)
	if apierrors.IsConflict(err) {
		return fmt.Errorf("delete gateway %q after conflict retries: %w", gatewayID, err)
	}
	return resourceError("delete", "gateway", gatewayID, err)
}
