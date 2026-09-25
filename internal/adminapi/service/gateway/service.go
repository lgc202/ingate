package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/repository"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// ErrInvalidArgument 表示网关创建参数不合法。
	ErrInvalidArgument = errors.New("invalid gateway argument")
	// ErrAlreadyExists 表示同名网关已存在。
	ErrAlreadyExists = errors.New("gateway already exists")
)

// CreateParams 保存创建 Gateway 所需的参数。
type CreateParams struct {
	Name        string
	DisplayName string
	Listeners   []gatewayv1alpha1.Listener
}

// Service 实现 Gateway 管理用例。
type Service struct {
	gateways *repository.Gateway
}

// New 创建 Gateway Service。
func New(gateways *repository.Gateway) *Service {
	return &Service{gateways: gateways}
}

// Create 创建 Gateway。
func (s *Service) Create(
	ctx context.Context,
	params CreateParams,
) (*gatewayv1alpha1.Gateway, error) {
	gateway := &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: params.Name},
		Spec: gatewayv1alpha1.GatewaySpec{
			DisplayName: params.DisplayName,
			Listeners:   params.Listeners,
		},
	}

	created, err := s.gateways.Create(ctx, gateway)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInvalid):
			return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
		case errors.Is(err, repository.ErrAlreadyExists):
			return nil, fmt.Errorf("%w: %w", ErrAlreadyExists, err)
		default:
			return nil, err
		}
	}

	return created, nil
}
