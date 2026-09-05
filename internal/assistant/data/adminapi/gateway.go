package adminapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
)

// ListGateways 查询当前配置域中的网关入口。
func (c *Client) ListGateways(
	ctx context.Context,
	query agenttool.ResourceListQuery,
) (agenttool.GatewayPage, error) {
	result, err := c.gateways.ListGateways(ctx, &adminv1.ListGatewaysRequest{
		Query: query.Text,
		Limit: query.Limit,
	})
	if err != nil {
		return agenttool.GatewayPage{}, fmt.Errorf("list gateways from Admin API: %w", err)
	}
	if result == nil {
		return agenttool.GatewayPage{}, errors.New("list gateways from Admin API: empty response")
	}

	gateways := make([]agenttool.Gateway, 0, len(result.GetGateways()))
	for _, gateway := range result.GetGateways() {
		if err := validateGatewayResponse(gateway); err != nil {
			return agenttool.GatewayPage{}, err
		}
		gateways = append(gateways, gatewayFromAPI(gateway))
	}
	return agenttool.GatewayPage{
		Items:   gateways,
		HasMore: result.GetNextCursor() != "",
	}, nil
}

func gatewayFromAPI(gateway *adminv1.Gateway) agenttool.Gateway {
	listeners := lo.Map(gateway.GetListeners(), func(listener *adminv1.GatewayListener, _ int) agenttool.Listener {
		return agenttool.Listener{
			Name:     listener.GetName(),
			Protocol: gatewayProtocol(listener.GetProtocol()),
			Port:     listener.GetPort(),
			Hostname: listener.GetHostname(),
		}
	})
	return agenttool.Gateway{
		ID:        gateway.GetId(),
		Name:      gateway.GetName(),
		Enabled:   gateway.GetEnabled(),
		State:     resourceState(gateway.GetState()),
		Message:   gateway.GetMessage(),
		Listeners: listeners,
	}
}

func gatewayProtocol(protocol adminv1.GatewayProtocol) string {
	switch protocol {
	case adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTP:
		return "http"
	case adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTPS:
		return "https"
	default:
		return "unknown"
	}
}

func validateGatewayResponse(gateway *adminv1.Gateway) error {
	if gateway == nil || !validResourceID(gateway.GetId()) || gateway.GetName() == "" ||
		len(gateway.GetListeners()) == 0 {
		return errors.New("invalid gateway returned by Admin API")
	}
	if !validResourceState(gateway.GetState()) {
		return fmt.Errorf("gateway %s returned an invalid resource state", gateway.GetId())
	}
	for _, listener := range gateway.GetListeners() {
		if listener == nil || listener.GetName() == "" || listener.GetPort() == 0 ||
			listener.GetProtocol() != adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTP &&
				listener.GetProtocol() != adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTPS {
			return fmt.Errorf("gateway %s returned an invalid listener", gateway.GetId())
		}
	}
	return nil
}
