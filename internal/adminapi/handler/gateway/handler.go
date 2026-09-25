package gateway

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	reasonInvalidArgument = "GATEWAY_INVALID_ARGUMENT"
	reasonAlreadyExists   = "GATEWAY_ALREADY_EXISTS"
)

var (
	_ adminv1.GatewayServiceServer     = (*Handler)(nil)
	_ adminv1.GatewayServiceHTTPServer = (*Handler)(nil)
)

// Handler 实现 Gateway 的 gRPC 和 HTTP 协议入口。
type Handler struct {
	adminv1.UnimplementedGatewayServiceServer

	service *gatewayservice.Service
}

// New 创建 Gateway Handler。
func New(service *gatewayservice.Service) *Handler {
	return &Handler{service: service}
}

// CreateGateway 创建 Gateway。
func (h *Handler) CreateGateway(
	ctx context.Context,
	req *adminv1.CreateGatewayRequest,
) (*adminv1.CreateGatewayReply, error) {
	created, err := h.service.Create(ctx, createParamsFromProto(req))
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.CreateGatewayReply{Gateway: gatewayToProto(created)}, nil
}

func createParamsFromProto(req *adminv1.CreateGatewayRequest) gatewayservice.CreateParams {
	listeners := make([]gatewayv1alpha1.Listener, len(req.GetListeners()))
	for i, listener := range req.GetListeners() {
		listeners[i] = gatewayv1alpha1.Listener{
			Name:     listener.GetName(),
			Protocol: protocolFromProto(listener.GetProtocol()),
			Port:     listener.GetPort(),
			Hostname: listener.GetHostname(),
		}
	}

	return gatewayservice.CreateParams{
		Name:        req.GetName(),
		DisplayName: req.GetDisplayName(),
		Listeners:   listeners,
	}
}

func protocolFromProto(protocol adminv1.ListenerProtocol) gatewayv1alpha1.Protocol {
	if protocol == adminv1.ListenerProtocol_LISTENER_PROTOCOL_HTTP {
		return gatewayv1alpha1.ProtocolHTTP
	}
	return ""
}

func gatewayToProto(gateway *gatewayv1alpha1.Gateway) *adminv1.Gateway {
	listeners := make([]*adminv1.Listener, len(gateway.Spec.Listeners))
	for i, listener := range gateway.Spec.Listeners {
		listeners[i] = &adminv1.Listener{
			Name:     listener.Name,
			Protocol: protocolToProto(listener.Protocol),
			Port:     listener.Port,
			Hostname: listener.Hostname,
		}
	}

	return &adminv1.Gateway{
		Name:        gateway.Name,
		DisplayName: gateway.Spec.DisplayName,
		Listeners:   listeners,
		CreateTime:  timestamppb.New(gateway.CreationTimestamp.Time),
	}
}

func protocolToProto(protocol gatewayv1alpha1.Protocol) adminv1.ListenerProtocol {
	if protocol == gatewayv1alpha1.ProtocolHTTP {
		return adminv1.ListenerProtocol_LISTENER_PROTOCOL_HTTP
	}
	return adminv1.ListenerProtocol_LISTENER_PROTOCOL_UNSPECIFIED
}

func toTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, gatewayservice.ErrInvalidArgument):
		return kratoserrors.BadRequest(reasonInvalidArgument, "网关参数不合法").WithCause(err)
	case errors.Is(err, gatewayservice.ErrAlreadyExists):
		return kratoserrors.Conflict(reasonAlreadyExists, "同名网关已存在").WithCause(err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}
