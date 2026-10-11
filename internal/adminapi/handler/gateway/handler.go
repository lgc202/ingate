package gateway

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const (
	reasonInvalidArgument = "GATEWAY_INVALID_ARGUMENT"
	reasonAlreadyExists   = "GATEWAY_ALREADY_EXISTS"
	reasonNotFound        = "GATEWAY_NOT_FOUND"
	reasonConflict        = "GATEWAY_CONFLICT"
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

// GetGateway 读取指定 Gateway。
func (h *Handler) GetGateway(
	ctx context.Context,
	req *adminv1.GetGatewayRequest,
) (*adminv1.GetGatewayReply, error) {
	gateway, err := h.service.Get(ctx, req.GetName())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.GetGatewayReply{Gateway: gatewayToProto(gateway)}, nil
}

// ListGateways 分页列出 Gateway。
func (h *Handler) ListGateways(
	ctx context.Context,
	req *adminv1.ListGatewaysRequest,
) (*adminv1.ListGatewaysReply, error) {
	page, err := h.service.List(ctx, gatewayservice.ListParams{
		PageSize:    req.GetPageSize(),
		PageNum:     req.GetPageNum(),
		DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	gateways := make([]*adminv1.Gateway, len(page.Items))
	for i, gateway := range page.Items {
		gateways[i] = gatewayToProto(gateway)
	}

	return &adminv1.ListGatewaysReply{Gateways: gateways, TotalCount: page.TotalCount}, nil
}

// UpdateGateway 完整替换 Gateway 的可编辑配置。
func (h *Handler) UpdateGateway(
	ctx context.Context,
	req *adminv1.UpdateGatewayRequest,
) (*adminv1.UpdateGatewayReply, error) {
	updated, err := h.service.Update(ctx, gatewayservice.UpdateParams{
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		DisplayName: req.GetDisplayName(),
		Listeners:   listenersFromProto(req.GetListeners()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.UpdateGatewayReply{Gateway: gatewayToProto(updated)}, nil
}

// DeleteGateway 删除指定 Gateway。
func (h *Handler) DeleteGateway(
	ctx context.Context,
	req *adminv1.DeleteGatewayRequest,
) (*adminv1.DeleteGatewayReply, error) {
	if err := h.service.Delete(ctx, req.GetName()); err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.DeleteGatewayReply{}, nil
}

func createParamsFromProto(req *adminv1.CreateGatewayRequest) gatewayservice.CreateParams {
	return gatewayservice.CreateParams{
		DisplayName: req.GetDisplayName(),
		Listeners:   listenersFromProto(req.GetListeners()),
	}
}

func listenersFromProto(items []*adminv1.Listener) []gatewayv1alpha1.Listener {
	listeners := make([]gatewayv1alpha1.Listener, len(items))
	for i, listener := range items {
		listeners[i] = gatewayv1alpha1.Listener{
			Name:     listener.GetName(),
			Protocol: protocolFromProto(listener.GetProtocol()),
			Port:     listener.GetPort(),
			Hostname: listener.GetHostname(),
		}
	}

	return listeners
}

func protocolFromProto(protocol adminv1.ListenerProtocol) gatewayv1alpha1.Protocol {
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
			Protocol: protocolToProto(listener.Protocol),
			Port:     listener.Port,
			Hostname: listener.Hostname,
		}
	}

	return &adminv1.Gateway{
		Name:        gateway.Name,
		DisplayName: gateway.DisplayName,
		Listeners:   listeners,
		CreateTime:  timestamppb.New(gateway.CreateTime),
		Version:     gateway.Version,
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
		return kratoserrors.Conflict(reasonAlreadyExists, "资源标识冲突，请重新创建").WithCause(err)
	case errors.Is(err, gatewayservice.ErrNotFound):
		return kratoserrors.NotFound(reasonNotFound, "网关不存在").WithCause(err)
	case errors.Is(err, gatewayservice.ErrConflict):
		return kratoserrors.Conflict(reasonConflict, "网关已被修改，请刷新后重试").WithCause(err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}
