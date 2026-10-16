package handler

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/errno"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

var (
	_ adminv1.GatewayServiceServer     = (*GatewayHandler)(nil)
	_ adminv1.GatewayServiceHTTPServer = (*GatewayHandler)(nil)
)

// GatewayHandler 实现 Gateway 的 gRPC 和 HTTP 协议入口。
type GatewayHandler struct {
	adminv1.UnimplementedGatewayServiceServer

	service *gatewayservice.Service
}

// NewGateway 创建网关协议入口，复用注入的业务服务。
func NewGateway(service *gatewayservice.Service) *GatewayHandler {
	return &GatewayHandler{service: service}
}

// CreateGateway 创建 Gateway。
func (h *GatewayHandler) CreateGateway(
	ctx context.Context,
	req *adminv1.CreateGatewayRequest,
) (*adminv1.CreateGatewayReply, error) {
	created, err := h.service.Create(ctx, gatewayservice.CreateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		Listeners:   gatewayListenersFromProto(req.GetListeners()),
	})
	if err != nil {
		return nil, toGatewayError(err)
	}

	return &adminv1.CreateGatewayReply{Gateway: gatewayToProto(created)}, nil
}

// GetGateway 读取指定 Gateway。
func (h *GatewayHandler) GetGateway(
	ctx context.Context,
	req *adminv1.GetGatewayRequest,
) (*adminv1.GetGatewayReply, error) {
	gateway, err := h.service.Get(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, toGatewayError(err)
	}

	return &adminv1.GetGatewayReply{Gateway: gatewayToProto(gateway)}, nil
}

// ListGateways 分页列出 Gateway。
func (h *GatewayHandler) ListGateways(
	ctx context.Context,
	req *adminv1.ListGatewaysRequest,
) (*adminv1.ListGatewaysReply, error) {
	page, err := h.service.List(ctx, gatewayservice.ListParams{
		Namespace:   req.GetNamespace(),
		PageSize:    req.GetPageSize(),
		PageNum:     req.GetPageNum(),
		DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, toGatewayError(err)
	}

	gateways := make([]*adminv1.Gateway, len(page.Items))
	for i, gateway := range page.Items {
		gateways[i] = gatewayToProto(gateway)
	}

	return &adminv1.ListGatewaysReply{Gateways: gateways, TotalCount: page.TotalCount}, nil
}

// UpdateGateway 完整替换 Gateway 的可编辑配置。
func (h *GatewayHandler) UpdateGateway(
	ctx context.Context,
	req *adminv1.UpdateGatewayRequest,
) (*adminv1.UpdateGatewayReply, error) {
	updated, err := h.service.Update(ctx, gatewayservice.UpdateParams{
		Namespace:   req.GetNamespace(),
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		DisplayName: req.GetDisplayName(),
		Listeners:   gatewayListenersFromProto(req.GetListeners()),
	})
	if err != nil {
		return nil, toGatewayError(err)
	}

	return &adminv1.UpdateGatewayReply{Gateway: gatewayToProto(updated)}, nil
}

// DeleteGateway 删除指定 Gateway。
func (h *GatewayHandler) DeleteGateway(
	ctx context.Context,
	req *adminv1.DeleteGatewayRequest,
) (*adminv1.DeleteGatewayReply, error) {
	if err := h.service.Delete(ctx, req.GetNamespace(), req.GetName()); err != nil {
		return nil, toGatewayError(err)
	}

	return &adminv1.DeleteGatewayReply{}, nil
}

func gatewayListenersFromProto(items []*adminv1.Listener) []gatewayv1alpha1.Listener {
	listeners := make([]gatewayv1alpha1.Listener, len(items))
	for i, listener := range items {
		listeners[i] = gatewayv1alpha1.Listener{
			Name:     listener.GetName(),
			Protocol: gatewayProtocolFromProto(listener.GetProtocol()),
			Port:     listener.GetPort(),
			Hostname: listener.GetHostname(),
		}
	}

	return listeners
}

func gatewayProtocolFromProto(protocol adminv1.ListenerProtocol) gatewayv1alpha1.Protocol {
	if protocol == adminv1.ListenerProtocol_LISTENER_PROTOCOL_HTTP {
		return gatewayv1alpha1.ProtocolHTTP
	}

	return ""
}

func gatewayToProto(gateway repository.GatewayInfo) *adminv1.Gateway {
	listeners := make([]*adminv1.Listener, len(gateway.Listeners))
	for i, listener := range gateway.Listeners {
		listeners[i] = &adminv1.Listener{
			Name:     listener.Name,
			Protocol: gatewayProtocolToProto(listener.Protocol),
			Port:     listener.Port,
			Hostname: listener.Hostname,
		}
	}

	return &adminv1.Gateway{
		Namespace:   gateway.Namespace,
		Name:        gateway.Name,
		DisplayName: gateway.DisplayName,
		Listeners:   listeners,
		CreateTime:  timestamppb.New(gateway.CreateTime),
		Version:     gateway.Version,
	}
}

func gatewayProtocolToProto(protocol gatewayv1alpha1.Protocol) adminv1.ListenerProtocol {
	if protocol == gatewayv1alpha1.ProtocolHTTP {
		return adminv1.ListenerProtocol_LISTENER_PROTOCOL_HTTP
	}

	return adminv1.ListenerProtocol_LISTENER_PROTOCOL_UNSPECIFIED
}

func toGatewayError(err error) error {
	switch {
	case errors.Is(err, gatewayservice.ErrInvalidArgument):
		return errno.ErrGatewayInvalidArgument.WithCause(err)
	case errors.Is(err, gatewayservice.ErrAlreadyExists):
		return errno.ErrGatewayAlreadyExists.WithCause(err)
	case errors.Is(err, gatewayservice.ErrNotFound):
		return errno.ErrGatewayNotFound.WithCause(err)
	case errors.Is(err, gatewayservice.ErrConflict):
		return errno.ErrGatewayConflict.WithCause(err)
	default:
		return errno.Fallback(err)
	}
}
