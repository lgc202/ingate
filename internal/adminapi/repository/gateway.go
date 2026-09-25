package repository

import (
	"context"
	"errors"
	"fmt"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// ErrInvalid 表示资源未通过 API Server 校验。
	ErrInvalid = errors.New("invalid gateway")
	// ErrAlreadyExists 表示同名资源已存在。
	ErrAlreadyExists = errors.New("gateway already exists")
)

// Gateway 提供 Gateway 资源的持久化操作。
type Gateway struct {
	client gatewayclient.GatewayInterface
}

// NewGateway 创建 Gateway Repository。
func NewGateway(client gatewayclient.GatewayInterface) *Gateway {
	return &Gateway{client: client}
}

// Create 创建 Gateway，并返回 API Server 保存后的资源。
func (r *Gateway) Create(
	ctx context.Context,
	gateway *gatewayv1alpha1.Gateway,
) (*gatewayv1alpha1.Gateway, error) {
	created, err := r.client.Create(ctx, gateway, metav1.CreateOptions{})
	if err != nil {
		switch {
		case apierrors.IsBadRequest(err), apierrors.IsInvalid(err):
			return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
		case apierrors.IsAlreadyExists(err):
			return nil, fmt.Errorf("%w: %w", ErrAlreadyExists, err)
		default:
			return nil, err
		}
	}

	return created, nil
}
